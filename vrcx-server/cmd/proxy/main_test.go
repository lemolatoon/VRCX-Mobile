package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/credentials"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/store"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/vrcapi"
)

const testEncryptionKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func TestLoginSessionAndProxyPassThrough(t *testing.T) {
	env := newProxyTestEnv(t)
	userID := "usr_proxy_allowed"
	env.allowUser(t, userID)

	var sawProxy atomic.Bool
	vrc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/1/auth/user":
			if r.Header.Get("Authorization") == "Basic "+basicAuth("allowed", "password") {
				http.SetCookie(w, &http.Cookie{Name: "auth", Value: "session-token", Path: "/"})
				writeJSON(t, w, http.StatusOK, map[string]string{
					"id":          userID,
					"displayName": "Allowed User",
				})
				return
			}
			if cookieValue(r, "auth") == "session-token" {
				writeJSON(t, w, http.StatusOK, map[string]string{
					"id":          userID,
					"displayName": "Allowed User",
				})
				return
			}
			writeJSON(t, w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/1/worlds/wrld_1":
			sawProxy.Store(true)
			if got := r.URL.RawQuery; got != "n=1" {
				t.Fatalf("proxy query = %q, want n=1", got)
			}
			if got := r.Header.Get("User-Agent"); got != vrcapi.UserAgent {
				t.Fatalf("user-agent = %q, want %q", got, vrcapi.UserAgent)
			}
			if cookieValue(r, "auth") != "session-token" {
				t.Fatalf("proxy request did not include VRChat auth cookie")
			}
			writeJSON(t, w, http.StatusOK, map[string]string{"id": "wrld_1"})
		default:
			t.Fatalf("unexpected VRChat request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer vrc.Close()

	proxy := env.startProxy(t, vrc.URL+"/api/1")
	defer proxy.Close()

	login := env.request(t, proxy, http.MethodPost, "/api/v1/auth/login", `{"username":"allowed","password":"password"}`, "")
	if login.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.StatusCode, login.Body)
	}
	sessionCookie := responseCookie(login, sessionCookieName)
	if sessionCookie == "" {
		t.Fatalf("login did not set %s cookie", sessionCookieName)
	}

	me := env.request(t, proxy, http.MethodGet, "/api/v1/auth/me", "", sessionCookie)
	if me.StatusCode != http.StatusOK {
		t.Fatalf("auth/me status = %d, body = %s", me.StatusCode, me.Body)
	}

	proxied := env.request(t, proxy, http.MethodGet, "/api/v1/proxy/worlds/wrld_1?n=1", "", sessionCookie)
	if proxied.StatusCode != http.StatusOK {
		t.Fatalf("proxy status = %d, body = %s", proxied.StatusCode, proxied.Body)
	}
	if !sawProxy.Load() {
		t.Fatalf("VRChat proxy endpoint was not called")
	}

	logout := env.request(t, proxy, http.MethodPost, "/api/v1/auth/logout", "", sessionCookie)
	if logout.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", logout.StatusCode, logout.Body)
	}
	afterLogout := env.request(t, proxy, http.MethodGet, "/api/v1/auth/me", "", sessionCookie)
	if afterLogout.StatusCode != http.StatusUnauthorized {
		t.Fatalf("auth/me after logout status = %d, want 401", afterLogout.StatusCode)
	}
}

func TestLoginRejectsUsersOutsideAllowlist(t *testing.T) {
	env := newProxyTestEnv(t)
	vrc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, map[string]string{"id": "usr_not_allowed"})
	}))
	defer vrc.Close()

	proxy := env.startProxy(t, vrc.URL+"/api/1")
	defer proxy.Close()

	resp := env.request(t, proxy, http.MethodPost, "/api/v1/auth/login", `{"username":"denied","password":"password"}`, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("login status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	if cookie := responseCookie(resp, sessionCookieName); cookie != "" {
		t.Fatalf("denied login set session cookie %q", cookie)
	}
}

func TestTwoFactorLoginMethods(t *testing.T) {
	for _, method := range []string{"totp", "emailotp"} {
		t.Run(method, func(t *testing.T) {
			env := newProxyTestEnv(t)
			userID := "usr_2fa_" + method
			env.allowUser(t, userID)

			var verified atomic.Bool
			vrc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/1/auth/user" && !verified.Load():
					writeJSON(t, w, http.StatusOK, map[string][]string{"requiresTwoFactorAuth": []string{method}})
				case r.Method == http.MethodPost && r.URL.Path == "/api/1/auth/twofactorauth/"+method+"/verify":
					if !strings.Contains(readBody(t, r), `"code":"123456"`) {
						t.Fatalf("2FA verify body did not include code")
					}
					verified.Store(true)
					http.SetCookie(w, &http.Cookie{Name: "auth", Value: "2fa-token-" + method, Path: "/"})
					writeJSON(t, w, http.StatusOK, map[string]bool{"verified": true})
				case r.Method == http.MethodGet && r.URL.Path == "/api/1/auth/user" && verified.Load():
					writeJSON(t, w, http.StatusOK, map[string]string{"id": userID})
				default:
					t.Fatalf("unexpected VRChat request: %s %s", r.Method, r.URL.String())
				}
			}))
			defer vrc.Close()

			proxy := env.startProxy(t, vrc.URL+"/api/1")
			defer proxy.Close()

			login := env.request(t, proxy, http.MethodPost, "/api/v1/auth/login", `{"username":"needs2fa","password":"password"}`, "")
			if login.StatusCode != http.StatusOK {
				t.Fatalf("login status = %d, body = %s", login.StatusCode, login.Body)
			}
			if !strings.Contains(login.Body, `"pending":"needs2fa"`) {
				t.Fatalf("login body missing pending username: %s", login.Body)
			}

			verify := env.request(t, proxy, http.MethodPost, "/api/v1/auth/2fa/"+method, `{"pending":"needs2fa","code":"123456"}`, "")
			if verify.StatusCode != http.StatusOK {
				t.Fatalf("2FA status = %d, body = %s", verify.StatusCode, verify.Body)
			}
			if cookie := responseCookie(verify, sessionCookieName); cookie == "" {
				t.Fatalf("2FA completion did not set session cookie")
			}
		})
	}
}

func TestAuthFailuresLockOutAfterFiveAttempts(t *testing.T) {
	env := newProxyTestEnv(t)
	vrc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, map[string]string{"error": "bad auth"})
	}))
	defer vrc.Close()

	proxy := env.startProxy(t, vrc.URL+"/api/1")
	defer proxy.Close()

	for i := 0; i < 5; i++ {
		resp := env.request(t, proxy, http.MethodPost, "/api/v1/auth/login", `{"username":"bad","password":"bad"}`, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401", i+1, resp.StatusCode)
		}
	}

	locked := env.request(t, proxy, http.MethodPost, "/api/v1/auth/login", `{"username":"bad","password":"bad"}`, "")
	if locked.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("locked status = %d, body = %s", locked.StatusCode, locked.Body)
	}
	if locked.Header.Get("Retry-After") == "" {
		t.Fatalf("locked response missing Retry-After")
	}
}

func TestLoginSurfacesUpstreamOutage(t *testing.T) {
	env := newProxyTestEnv(t)
	vrc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{
				"message":     `"VRChat API services are currently unavailable."`,
				"status_code": 503,
			},
		})
	}))
	defer vrc.Close()

	proxy := env.startProxy(t, vrc.URL+"/api/1")
	defer proxy.Close()

	resp := env.request(t, proxy, http.MethodPost, "/api/v1/auth/login", `{"username":"any","password":"any"}`, "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("login status = %d, body = %s; want 503", resp.StatusCode, resp.Body)
	}
	if strings.Contains(resp.Body, "bad upstream response") {
		t.Fatalf("login body should not say 'bad upstream response': %s", resp.Body)
	}
	if !strings.Contains(resp.Body, "temporarily unavailable") {
		t.Fatalf("login body should mention 'temporarily unavailable': %s", resp.Body)
	}
}

func TestAgentTokenIngestAndListGameLog(t *testing.T) {
	env := newProxyTestEnv(t)
	userID := "usr_agent_allowed"
	env.allowUser(t, userID)

	vrc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, map[string]string{"id": userID, "displayName": "Agent User"})
	}))
	defer vrc.Close()

	proxy := env.startProxy(t, vrc.URL+"/api/1")
	defer proxy.Close()

	login := env.request(t, proxy, http.MethodPost, "/api/v1/auth/login", `{"username":"agent","password":"password"}`, "")
	if login.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.StatusCode, login.Body)
	}
	sessionCookie := responseCookie(login, sessionCookieName)

	created := env.request(t, proxy, http.MethodPost, "/api/v1/agent-tokens", `{"name":"test pc"}`, sessionCookie)
	if created.StatusCode != http.StatusOK {
		t.Fatalf("create token status = %d, body = %s", created.StatusCode, created.Body)
	}
	var tokenResp struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(created.Body), &tokenResp); err != nil {
		t.Fatalf("unmarshal token: %v", err)
	}
	if tokenResp.Token == "" {
		t.Fatalf("token response did not include plaintext token")
	}

	body := `{"source_id":"src_test","entries":[{"log_file":"output_log.txt","line_offset":42,"created_at":"2026-06-18T03:00:00Z","type":"Unknown","payload":{"message":"hello"},"raw_line":"hello"}]}`
	req, err := http.NewRequest(http.MethodPost, proxy.URL+"/api/v1/gamelog/ingest", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new ingest request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	resp, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatalf("ingest request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ingest status = %d", resp.StatusCode)
	}

	list := env.request(t, proxy, http.MethodGet, "/api/v1/gamelog?type=Unknown", "", sessionCookie)
	if list.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", list.StatusCode, list.Body)
	}
	if !strings.Contains(list.Body, `"raw_line":"hello"`) {
		t.Fatalf("list body missing gamelog entry: %s", list.Body)
	}
}

type proxyTestEnv struct {
	db   *pgxpool.Pool
	cred *credentials.Store
}

type testResponse struct {
	StatusCode int
	Header     http.Header
	Body       string
	Cookies    []*http.Cookie
}

func newProxyTestEnv(t *testing.T) *proxyTestEnv {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(db.Close)

	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	if _, err := db.Exec(context.Background(), `TRUNCATE allowed_users, sessions, vrchat_credentials, rate_limit_attempts, agent_tokens, gamelog_entries`); err != nil {
		t.Fatalf("truncate test db: %v", err)
	}

	cred, err := credentials.New(db, testEncryptionKey)
	if err != nil {
		t.Fatalf("credentials store: %v", err)
	}

	return &proxyTestEnv{db: db, cred: cred}
}

func (e *proxyTestEnv) allowUser(t *testing.T, userID string) {
	t.Helper()
	_, err := e.db.Exec(context.Background(),
		`INSERT INTO allowed_users (vrchat_user_id, note) VALUES ($1, 'test')`,
		userID,
	)
	if err != nil {
		t.Fatalf("allow user: %v", err)
	}
}

func (e *proxyTestEnv) startProxy(t *testing.T, apiBaseURL string) *httptest.Server {
	t.Helper()
	server := newProxyServerWithClientFactory(e.db, e.cred, func() (*vrcapi.Client, error) {
		return vrcapi.NewClientWithBaseURL(apiBaseURL)
	})
	return httptest.NewServer(server)
}

func (e *proxyTestEnv) request(t *testing.T, server *httptest.Server, method, path, body, cookie string) *testResponse {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return &testResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       string(respBody),
		Cookies:    resp.Cookies(),
	}
}

func responseCookie(resp *testResponse, name string) string {
	for _, c := range resp.Cookies {
		if c.Name == name && c.Value != "" {
			return c.Name + "=" + c.Value
		}
	}
	return ""
}

func cookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	defer r.Body.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return buf.String()
}
