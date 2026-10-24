package blink

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"blinkcli/internal/config"
)

const (
	baseURL          = "https://blinkit.com"
	orderHistoryPath = "/v1/layout/order_history"
	orderCountPath   = "/v1/order_count"
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// Client calls Blinkit web endpoints using a captured session.
type Client struct {
	HTTP    *http.Client
	Session *config.Session
}

func NewClient(session *config.Session) *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 20 * time.Second},
		Session: session,
	}
}

// OrderCount returns realtime delivered/live/cancelled counts.
func (c *Client) OrderCount(ctx context.Context) (OrderCount, error) {
	if c.Session == nil {
		return OrderCount{}, errors.New("missing session")
	}
	if err := c.ensureCookies(ctx); err != nil {
		return OrderCount{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+orderCountPath, nil)
	if err != nil {
		return OrderCount{}, err
	}
	applyHeaders(req, c.Session)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return OrderCount{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OrderCount{}, fmt.Errorf("order_count request failed: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return OrderCount{}, err
	}
	return parseOrderCount(body, c.Session.UserID)
}

// OrderHistory fetches one page ("" = first) and returns the next page path ("" = last).
func (c *Client) OrderHistory(ctx context.Context, path string) ([]Order, string, error) {
	if c.Session == nil {
		return nil, "", errors.New("missing session")
	}
	path, err := PagePath(path)
	if err != nil {
		return nil, "", err
	}
	if err := c.ensureCookies(ctx); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, nil)
	if err != nil {
		return nil, "", err
	}
	applyHeaders(req, c.Session)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("order_history request failed: %s", resp.Status)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return ParseOrderHistory(respBody, time.Now())
}

// apiHeaders returns the Blinkit app headers derived from the session.
// Browser-controlled headers (origin, referer, user-agent, cookie) are excluded.
func apiHeaders(session *config.Session) map[string]string {
	h := map[string]string{
		"Accept":                "application/json",
		"Content-Type":          "application/json",
		"app_client":            "consumer_web",
		"platform":              "desktop_web",
		"x-age-consent-granted": "false",
	}
	set := func(k, v string) {
		if v != "" {
			h[k] = v
		}
	}
	set("access_token", session.AccessToken)
	set("auth_key", session.AuthKey)
	set("device_id", session.DeviceID)
	set("session_uuid", session.SessionID)
	if session.Lat != 0 {
		h["lat"] = fmt.Sprintf("%f", session.Lat)
	}
	if session.Lon != 0 {
		h["lon"] = fmt.Sprintf("%f", session.Lon)
	}
	set("web_app_version", session.WebAppVersion)
	set("app_version", session.AppVersion)
	set("rn_bundle_version", session.RNBundleVersion)
	return h
}

// PagePath allows only blinkit.com order_history URLs, so session headers stay on that endpoint.
func PagePath(raw string) (string, error) {
	if raw == "" {
		return orderHistoryPath, nil
	}
	bad := fmt.Errorf("unexpected pagination url %q", raw)
	// Browsers treat a backslash like "/", so "/\evil.example" would resolve off-origin.
	if strings.ContainsRune(raw, '\\') || strings.HasPrefix(raw, "//") {
		return "", bad
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return "", bad
	}
	if u.Scheme != "" || u.Host != "" {
		host := u.Hostname()
		if u.Scheme != "https" || (host != "blinkit.com" && host != "www.blinkit.com") || (u.Port() != "" && u.Port() != "443") {
			return "", bad
		}
	}
	if u.Path != orderHistoryPath {
		return "", bad
	}
	if u.RawQuery == "" {
		return u.Path, nil
	}
	return u.Path + "?" + u.RawQuery, nil
}

func applyHeaders(req *http.Request, session *config.Session) {
	for k, v := range apiHeaders(session) {
		req.Header.Set(k, v)
	}
	req.Header.Set("origin", baseURL)
	req.Header.Set("referer", baseURL+"/account/orders")
	ua := session.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	req.Header.Set("user-agent", ua)
	if len(session.Cookies) > 0 {
		req.Header.Set("Cookie", cookieHeader(session.Cookies))
	}
}

func cookieHeader(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for k, v := range cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, "; ")
}

func (c *Client) ensureCookies(ctx context.Context) error {
	if c.Session == nil {
		return errors.New("missing session")
	}
	config.PopulateDerivedCookies(c.Session)
	if hasCookie(c.Session.Cookies, "__cf_bm") {
		return nil
	}
	if err := c.bootstrapCookies(ctx); err != nil {
		return err
	}
	config.PopulateDerivedCookies(c.Session)
	return nil
}

func (c *Client) bootstrapCookies(ctx context.Context) error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
	if err != nil {
		return err
	}
	ua := c.Session.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	u, _ := url.Parse(baseURL)
	for _, ck := range jar.Cookies(u) {
		c.Session.Cookies[ck.Name] = ck.Value
	}
	return nil
}

func hasCookie(cookies map[string]string, name string) bool {
	if cookies == nil {
		return false
	}
	_, ok := cookies[name]
	return ok
}
