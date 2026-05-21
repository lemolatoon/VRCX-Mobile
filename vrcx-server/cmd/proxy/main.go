// Command proxy is the HTTP server that:
//   - Authenticates PWA users via VRChat credentials + allowlist
//   - Issues session cookies
//   - Forwards authenticated requests to the VRChat API
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/auth"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/ratelimit"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/session"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/store"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/vrcapi"
)

const sessionCookieName = "vrcx_session"

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Migrate(ctx, db); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}

	allowlist := auth.NewAllowlist(db)
	sessions := session.NewStore(db)
	// In-process VRChat HTTP clients indexed by vrchat_user_id.
	// On restart clients are re-created from cookies stored in Postgres.
	clients := make(map[string]*vrcapi.Client)

	// Rate limiter: 1 req / 2s burst 5 per IP for auth endpoints
	authLimiter := ratelimit.New(rate.Every(2*time.Second), 5)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true, LogMethod: true, LogURI: true,
		LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
			slog.Info("request", "method", v.Method, "uri", v.URI, "status", v.Status)
			return nil
		},
	}))
	e.Use(middleware.Recover())

	// Health probe
	e.GET("/healthz", func(c echo.Context) error {
		if err := db.Ping(ctx); err != nil {
			return c.String(http.StatusServiceUnavailable, "db down")
		}
		return c.String(http.StatusOK, "ok")
	})

	// Auth endpoints
	e.POST("/api/v1/auth/login", func(c echo.Context) error {
		ipKey := ratelimit.IPKey(c.Request())
		if ok, retryAfter := authLimiter.Allow(ipKey); !ok {
			c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
			return c.JSON(http.StatusTooManyRequests, echo.Map{"error": "rate limited"})
		}

		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.Bind(&req); err != nil || req.Username == "" || req.Password == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "username and password required"})
		}

		client, err := vrcapi.NewClient()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal error"})
		}

		encoded := basicAuth(req.Username, req.Password)
		resp, err := client.Do(c.Request().Context(), "GET", "auth/user", nil, map[string]string{
			"Authorization": "Basic " + encoded,
		})
		if err != nil {
			authLimiter.RecordFailure(ipKey)
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "upstream error"})
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusUnauthorized {
			authLimiter.RecordFailure(ipKey)
			// Uniform delay to prevent timing attacks
			time.Sleep(300 * time.Millisecond)
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid credentials"})
		}

		// Parse the VRChat response to get userId and check 2FA requirement
		var vrcResp map[string]json.RawMessage
		if err := json.Unmarshal(body, &vrcResp); err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "bad upstream response"})
		}

		// Check for 2FA requirement
		if tfa, ok := vrcResp["requiresTwoFactorAuth"]; ok {
			// Forward the 2FA prompt to the PWA; stash the client by a temp key
			var tfaMethods []string
			_ = json.Unmarshal(tfa, &tfaMethods)
			// Store temp client under username (cleared on 2FA completion / timeout)
			clients["pending:"+req.Username] = client
			return c.JSON(http.StatusOK, echo.Map{
				"requiresTwoFactorAuth": tfaMethods,
				"pending":               req.Username,
			})
		}

		// Extract the userId
		userID, err := extractUserID(vrcResp)
		if err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "bad upstream response"})
		}

		// Allowlist check — uniform timing regardless of outcome
		allowed, err := allowlist.IsAllowed(c.Request().Context(), userID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal error"})
		}
		time.Sleep(100 * time.Millisecond) // constant-time guard
		if !allowed {
			authLimiter.RecordFailure(ipKey)
			return c.JSON(http.StatusForbidden, echo.Map{"error": "not authorized"})
		}

		authLimiter.RecordSuccess(ipKey)
		clients[userID] = client
		sess, err := sessions.Create(c.Request().Context(), userID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "session error"})
		}
		setSessionCookie(c, sess.ID, sess.ExpiresAt)
		return c.JSON(http.StatusOK, echo.Map{"userId": userID, "currentUser": vrcResp})
	})

	e.POST("/api/v1/auth/2fa/:method", func(c echo.Context) error {
		ipKey := ratelimit.IPKey(c.Request())
		if ok, _ := authLimiter.Allow(ipKey); !ok {
			return c.JSON(http.StatusTooManyRequests, echo.Map{"error": "rate limited"})
		}

		var req struct {
			Code    string `json:"code"`
			Pending string `json:"pending"`
		}
		if err := c.Bind(&req); err != nil || req.Code == "" || req.Pending == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "code and pending required"})
		}

		client, ok := clients["pending:"+req.Pending]
		if !ok {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "no pending login"})
		}

		method := c.Param("method") // otp, totp, emailotp
		payload := fmt.Sprintf(`{"code":"%s"}`, req.Code)
		resp, err := client.Do(c.Request().Context(), "POST",
			"auth/twofactorauth/"+method+"/verify",
			strings.NewReader(payload),
			map[string]string{"Content-Type": "application/json"},
		)
		if err != nil || resp.StatusCode != http.StatusOK {
			authLimiter.RecordFailure(ipKey)
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "2FA verification failed"})
		}
		resp.Body.Close()

		// Now fetch /auth/user to get the userId
		userResp, err := client.Do(c.Request().Context(), "GET", "auth/user", nil, nil)
		if err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "upstream error"})
		}
		defer userResp.Body.Close()
		body, _ := io.ReadAll(userResp.Body)

		var vrcResp map[string]json.RawMessage
		if err := json.Unmarshal(body, &vrcResp); err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "bad upstream response"})
		}
		userID, err := extractUserID(vrcResp)
		if err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "bad upstream response"})
		}

		allowed, err := allowlist.IsAllowed(c.Request().Context(), userID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal error"})
		}
		time.Sleep(100 * time.Millisecond)
		if !allowed {
			authLimiter.RecordFailure(ipKey)
			delete(clients, "pending:"+req.Pending)
			return c.JSON(http.StatusForbidden, echo.Map{"error": "not authorized"})
		}

		authLimiter.RecordSuccess(ipKey)
		delete(clients, "pending:"+req.Pending)
		clients[userID] = client
		sess, err := sessions.Create(c.Request().Context(), userID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "session error"})
		}
		setSessionCookie(c, sess.ID, sess.ExpiresAt)
		return c.JSON(http.StatusOK, echo.Map{"userId": userID, "currentUser": vrcResp})
	})

	e.POST("/api/v1/auth/logout", func(c echo.Context) error {
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil {
			return c.JSON(http.StatusOK, echo.Map{"ok": true})
		}
		_ = sessions.Delete(c.Request().Context(), cookie.Value)
		c.SetCookie(&http.Cookie{
			Name: sessionCookieName, Value: "",
			MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
		})
		return c.JSON(http.StatusOK, echo.Map{"ok": true})
	})

	// Transparent proxy to VRChat API (session-authenticated)
	e.Any("/api/v1/proxy/*", func(c echo.Context) error {
		sess, err := requireSession(c, sessions)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
		}

		client, ok := clients[sess.VRChatUserID]
		if !ok {
			// Re-create client from stored cookies (Phase 4: load from Postgres)
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "session expired, please re-login"})
		}

		vrcPath := strings.TrimPrefix(c.Param("*"), "/")
		reqURL := vrcPath
		if q := c.QueryString(); q != "" {
			reqURL += "?" + q
		}

		var body io.Reader
		if c.Request().Body != nil {
			body = c.Request().Body
			defer c.Request().Body.Close()
		}

		headers := map[string]string{}
		if ct := c.Request().Header.Get("Content-Type"); ct != "" {
			headers["Content-Type"] = ct
		}

		resp, err := client.Do(c.Request().Context(), c.Request().Method, reqURL, body, headers)
		if err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "upstream error"})
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		return c.Blob(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("starting proxy", "port", port)
	if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func requireSession(c echo.Context, sessions *session.Store) (*session.Session, error) {
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}
	sess, err := sessions.Get(c.Request().Context(), cookie.Value)
	if err != nil || sess == nil {
		return nil, fmt.Errorf("invalid or expired session")
	}
	return sess, nil
}

func setSessionCookie(c echo.Context, id string, expires time.Time) {
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

func extractUserID(resp map[string]json.RawMessage) (string, error) {
	raw, ok := resp["id"]
	if !ok {
		return "", fmt.Errorf("no 'id' field in response")
	}
	var id string
	if err := json.Unmarshal(raw, &id); err != nil {
		return "", err
	}
	return id, nil
}
