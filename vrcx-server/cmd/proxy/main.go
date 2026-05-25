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
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/auth"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/credentials"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/feed"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/ratelimit"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/session"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/store"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/vrcapi"
)

const sessionCookieName = "vrcx_session"

// clientRegistry holds per-user vrcapi.Client instances in memory,
// backed by Postgres for persistence across restarts.
type clientRegistry struct {
	mu        sync.RWMutex
	clients   map[string]*vrcapi.Client // key: vrchat_user_id
	creds     *credentials.Store
	newClient func() (*vrcapi.Client, error)
}

func newClientRegistry(creds *credentials.Store) *clientRegistry {
	return &clientRegistry{
		clients:   make(map[string]*vrcapi.Client),
		creds:     creds,
		newClient: vrcapi.NewClient,
	}
}

// get returns the in-memory client if present; otherwise loads cookies from
// Postgres and rebuilds a client. Returns nil if no credentials are stored.
func (r *clientRegistry) get(ctx context.Context, vrchatUserID string) (*vrcapi.Client, error) {
	r.mu.RLock()
	c, ok := r.clients[vrchatUserID]
	r.mu.RUnlock()
	if ok {
		return c, nil
	}

	// Attempt to restore from Postgres
	cookies, err := r.creds.Load(ctx, vrchatUserID)
	if err != nil || len(cookies) == 0 {
		return nil, err
	}
	client, err := r.newClient()
	if err != nil {
		return nil, err
	}
	client.SetCookies(cookies)

	r.mu.Lock()
	r.clients[vrchatUserID] = client
	r.mu.Unlock()
	return client, nil
}

// set stores a client in memory and saves its cookies to Postgres.
func (r *clientRegistry) set(ctx context.Context, vrchatUserID string, client *vrcapi.Client) error {
	r.mu.Lock()
	r.clients[vrchatUserID] = client
	r.mu.Unlock()
	return r.creds.Save(ctx, vrchatUserID, client.GetCookies())
}

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	encKey := os.Getenv("COOKIE_ENCRYPTION_KEY")
	if encKey == "" {
		slog.Error("COOKIE_ENCRYPTION_KEY is required (base64-encoded 32 bytes)")
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

	credStore, err := credentials.New(db, encKey)
	if err != nil {
		slog.Error("credentials store", "error", err)
		os.Exit(1)
	}

	e := newProxyServer(db, credStore)
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

func newProxyServer(db *pgxpool.Pool, credStore *credentials.Store) *echo.Echo {
	return newProxyServerWithClientFactory(db, credStore, vrcapi.NewClient)
}

func newProxyServerWithClientFactory(
	db *pgxpool.Pool,
	credStore *credentials.Store,
	newClient func() (*vrcapi.Client, error),
) *echo.Echo {
	allowlist := auth.NewAllowlist(db)
	sessions := session.NewStore(db)
	clients := newClientRegistry(credStore)
	clients.newClient = newClient
	feedStore := feed.NewStore(db)

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
		if err := db.Ping(c.Request().Context()); err != nil {
			return c.String(http.StatusServiceUnavailable, "db down")
		}
		return c.String(http.StatusOK, "ok")
	})

	// ── Auth endpoints ──────────────────────────────────────────────────────

	e.GET("/api/v1/auth/me", func(c echo.Context) error {
		sess, err := requireSession(c, sessions)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
		}
		client, err := clients.get(c.Request().Context(), sess.VRChatUserID)
		if err != nil || client == nil {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "session expired, please re-login"})
		}
		resp, err := client.Do(c.Request().Context(), "GET", "auth/user", nil, nil)
		if err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "upstream error"})
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return c.Blob(resp.StatusCode, "application/json", body)
	})

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

		client, err := clients.newClient()
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
			time.Sleep(300 * time.Millisecond) // constant-time guard
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid credentials"})
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			slog.Warn("vrchat upstream error", "status", resp.StatusCode, "path", "auth/user")
			return c.JSON(resp.StatusCode, echo.Map{"error": upstreamMessage(resp.StatusCode)})
		}

		var vrcResp map[string]json.RawMessage
		if err := json.Unmarshal(body, &vrcResp); err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "bad upstream response"})
		}

		// 2FA required — stash client under pending key
		if tfa, ok := vrcResp["requiresTwoFactorAuth"]; ok {
			var tfaMethods []string
			_ = json.Unmarshal(tfa, &tfaMethods)
			clients.mu.Lock()
			clients.clients["pending:"+req.Username] = client
			clients.mu.Unlock()
			return c.JSON(http.StatusOK, echo.Map{
				"requiresTwoFactorAuth": tfaMethods,
				"pending":               req.Username,
			})
		}

		userID, err := extractUserID(vrcResp)
		if err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "bad upstream response"})
		}

		return finishLogin(c, clients, sessions, allowlist, authLimiter, ipKey, userID, client, vrcResp)
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

		clients.mu.RLock()
		client, ok := clients.clients["pending:"+req.Pending]
		clients.mu.RUnlock()
		if !ok {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "no pending login"})
		}

		method := c.Param("method")
		payload := fmt.Sprintf(`{"code":"%s"}`, req.Code)
		verifyResp, err := client.Do(c.Request().Context(), "POST",
			"auth/twofactorauth/"+method+"/verify",
			strings.NewReader(payload),
			map[string]string{"Content-Type": "application/json"},
		)
		if err != nil {
			authLimiter.RecordFailure(ipKey)
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "upstream error"})
		}
		defer verifyResp.Body.Close()
		if verifyResp.StatusCode >= 500 {
			slog.Warn("vrchat upstream error", "status", verifyResp.StatusCode, "path", "2fa/verify")
			return c.JSON(verifyResp.StatusCode, echo.Map{"error": upstreamMessage(verifyResp.StatusCode)})
		}
		if verifyResp.StatusCode != http.StatusOK {
			authLimiter.RecordFailure(ipKey)
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "2FA verification failed"})
		}

		clients.mu.Lock()
		delete(clients.clients, "pending:"+req.Pending)
		clients.mu.Unlock()

		userResp, err := client.Do(c.Request().Context(), "GET", "auth/user", nil, nil)
		if err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "upstream error"})
		}
		defer userResp.Body.Close()
		body, _ := io.ReadAll(userResp.Body)

		if userResp.StatusCode < 200 || userResp.StatusCode >= 300 {
			slog.Warn("vrchat upstream error", "status", userResp.StatusCode, "path", "2fa/auth/user")
			return c.JSON(userResp.StatusCode, echo.Map{"error": upstreamMessage(userResp.StatusCode)})
		}

		var vrcResp map[string]json.RawMessage
		if err := json.Unmarshal(body, &vrcResp); err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "bad upstream response"})
		}
		userID, err := extractUserID(vrcResp)
		if err != nil {
			return c.JSON(http.StatusBadGateway, echo.Map{"error": "bad upstream response"})
		}

		return finishLogin(c, clients, sessions, allowlist, authLimiter, ipKey, userID, client, vrcResp)
	})

	e.POST("/api/v1/auth/logout", func(c echo.Context) error {
		cookie, err := c.Cookie(sessionCookieName)
		if err == nil {
			_ = sessions.Delete(c.Request().Context(), cookie.Value)
		}
		c.SetCookie(&http.Cookie{
			Name: sessionCookieName, Value: "",
			MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
		})
		return c.JSON(http.StatusOK, echo.Map{"ok": true})
	})

	// ── Feed read API ───────────────────────────────────────────────────────

	e.GET("/api/v1/feed", func(c echo.Context) error {
		sess, err := requireSession(c, sessions)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
		}

		// Parse type filter (comma-separated)
		var types []string
		if raw := c.QueryParam("type"); raw != "" {
			for _, t := range strings.Split(raw, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					types = append(types, t)
				}
			}
		}

		// Parse before cursor ("createdAt:id")
		var before *feed.Cursor
		if raw := c.QueryParam("before"); raw != "" {
			parts := strings.SplitN(raw, ":", 2)
			if len(parts) == 2 {
				t, terr := time.Parse(time.RFC3339Nano, parts[0])
				var id int64
				_, ierr := fmt.Sscanf(parts[1], "%d", &id)
				if terr == nil && ierr == nil {
					before = &feed.Cursor{CreatedAt: t, ID: id}
				}
			}
		}

		limit := 50
		if raw := c.QueryParam("limit"); raw != "" {
			fmt.Sscanf(raw, "%d", &limit)
		}

		items, nextCursor, err := feedStore.List(c.Request().Context(), sess.VRChatUserID, feed.ListOpts{
			Types:  types,
			Before: before,
			Limit:  limit,
		})
		if err != nil {
			slog.Warn("feed.List", "err", err)
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal error"})
		}

		var nextCursorStr *string
		if nextCursor != nil {
			s := nextCursor.CreatedAt.UTC().Format(time.RFC3339Nano) + ":" + fmt.Sprintf("%d", nextCursor.ID)
			nextCursorStr = &s
		}

		return c.JSON(http.StatusOK, echo.Map{
			"entries":     items,
			"next_cursor": nextCursorStr,
		})
	})

	// ── Transparent proxy to VRChat API ────────────────────────────────────

	e.Any("/api/v1/proxy/*", func(c echo.Context) error {
		sess, err := requireSession(c, sessions)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
		}

		client, err := clients.get(c.Request().Context(), sess.VRChatUserID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal error"})
		}
		if client == nil {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "session expired, please re-login"})
		}

		vrcPath := strings.TrimPrefix(c.Param("*"), "/")
		reqURL := vrcPath
		if q := c.QueryString(); q != "" {
			reqURL += "?" + q
		}

		var body io.Reader
		if c.Request().ContentLength != 0 {
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

		// Persist updated cookies after every proxied request (e.g. cookie refresh)
		if err := credStore.Save(c.Request().Context(), sess.VRChatUserID, client.GetCookies()); err != nil {
			slog.Warn("cookie save", "user", sess.VRChatUserID, "error", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		return c.Blob(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	})

	return e
}

// finishLogin checks the allowlist, creates a session, and sets the cookie.
func finishLogin(
	c echo.Context,
	clients *clientRegistry,
	sessions *session.Store,
	allowlist *auth.Allowlist,
	limiter *ratelimit.Limiter,
	ipKey, userID string,
	client *vrcapi.Client,
	vrcResp map[string]json.RawMessage,
) error {
	allowed, err := allowlist.IsAllowed(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal error"})
	}
	time.Sleep(100 * time.Millisecond) // constant-time guard against allowlist probing
	if !allowed {
		limiter.RecordFailure(ipKey)
		return c.JSON(http.StatusForbidden, echo.Map{"error": "not authorized"})
	}

	limiter.RecordSuccess(ipKey)
	if err := clients.set(c.Request().Context(), userID, client); err != nil {
		slog.Warn("save credentials", "user", userID, "error", err)
	}

	sess, err := sessions.Create(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "session error"})
	}
	setSessionCookie(c, sess.ID, sess.ExpiresAt)
	return c.JSON(http.StatusOK, echo.Map{"userId": userID, "currentUser": vrcResp})
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
		return "", fmt.Errorf("no 'id' field in VRChat response")
	}
	var id string
	if err := json.Unmarshal(raw, &id); err != nil {
		return "", err
	}
	return id, nil
}

func upstreamMessage(status int) string {
	if status >= 500 {
		return "VRChat is temporarily unavailable, please try again later (status.vrchat.com)"
	}
	return "VRChat request failed"
}
