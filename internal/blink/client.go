package blink

import (
	"bytes"
	"context"
	"encoding/json"
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

// OrderHistory fetches one page of order history.
// If pageSize is 0, an empty body is sent (matches observed web call).
func (c *Client) OrderHistory(ctx context.Context, page, pageSize int) ([]Order, error) {
	if c.Session == nil {
		return nil, errors.New("missing session")
	}
	if err := c.ensureCookies(ctx); err != nil {
		return nil, err
	}
	var body io.Reader
	if pageSize > 0 || page > 1 {
		payload := map[string]int{
			"page":      page,
			"page_size": pageSize,
		}
		buf, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+orderHistoryPath, body)
	if err != nil {
		return nil, err
	}
	applyHeaders(req, c.Session)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("order_history request failed: %s", resp.Status)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return ParseOrderHistory(respBody, time.Now())
}

func applyHeaders(req *http.Request, session *config.Session) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("app_client", "consumer_web")
	req.Header.Set("platform", "desktop_web")
	req.Header.Set("x-age-consent-granted", "false")
	req.Header.Set("origin", baseURL)
	req.Header.Set("referer", baseURL+"/account/orders")
	if session.AccessToken != "" {
		req.Header.Set("access_token", session.AccessToken)
	}
	if session.AuthKey != "" {
		req.Header.Set("auth_key", session.AuthKey)
	}
	if session.DeviceID != "" {
		req.Header.Set("device_id", session.DeviceID)
	}
	if session.SessionID != "" {
		req.Header.Set("session_uuid", session.SessionID)
	}
	if session.Lat != 0 {
		req.Header.Set("lat", fmt.Sprintf("%f", session.Lat))
	}
	if session.Lon != 0 {
		req.Header.Set("lon", fmt.Sprintf("%f", session.Lon))
	}
	if session.WebAppVersion != "" {
		req.Header.Set("web_app_version", session.WebAppVersion)
	}
	if session.AppVersion != "" {
		req.Header.Set("app_version", session.AppVersion)
	}
	if session.RNBundleVersion != "" {
		req.Header.Set("rn_bundle_version", session.RNBundleVersion)
	}
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
