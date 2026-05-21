// Package vrcapi implements an HTTP client for the VRChat API.
// It manages per-user cookie jars and injects the required User-Agent header.
// This mirrors the behavior of Dotnet/WebApi.cs in the Electron desktop app.
package vrcapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

const (
	BaseURL   = "https://api.vrchat.cloud/api/1"
	UserAgent = "VRCX-Mobile/0.1.0 (https://github.com/lemolatoon/vrcx-mobile)"
)

// Client is an HTTP client scoped to a single VRChat user session.
// Each authenticated user gets their own Client with its own cookie jar.
type Client struct {
	http      *http.Client
	jar       http.CookieJar
	baseURL   string
	userAgent string
}

// NewClient creates a Client with a fresh cookie jar.
func NewClient() (*Client, error) {
	return NewClientWithBaseURL(BaseURL)
}

// NewClientWithBaseURL creates a Client pointed at a custom API base URL.
// Tests use this to route VRChat API calls to an httptest.Server.
func NewClientWithBaseURL(baseURL string) (*Client, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, fmt.Errorf("cookiejar: %w", err)
	}
	transport := &http.Transport{
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	httpClient := &http.Client{
		Jar:       jar,
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	return &Client{
		http:      httpClient,
		jar:       jar,
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: UserAgent,
	}, nil
}

// Do executes an HTTP request against the VRChat API.
func (c *Client) Do(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	rawURL := c.baseURL + "/" + strings.TrimPrefix(path, "/")
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.http.Do(req)
}

// GetCookies serialises the jar's cookies for the VRChat domain.
func (c *Client) GetCookies() []*http.Cookie {
	u, _ := url.Parse(c.baseURL)
	return c.jar.Cookies(u)
}

// SetCookies loads cookies back into the jar (used when restoring a session).
func (c *Client) SetCookies(cookies []*http.Cookie) {
	u, _ := url.Parse(c.baseURL)
	c.jar.SetCookies(u, cookies)
}
