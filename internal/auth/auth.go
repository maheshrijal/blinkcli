package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"blinkcli/internal/config"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
)

const (
	blinkitURL       = "https://blinkit.com/"
	blinkitOrigin    = "https://blinkit.com"
	loginPollDelay   = 2 * time.Second
	loginTimeout     = 8 * time.Minute
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

type authPayload struct {
	AccessToken string `json:"accessToken"`
	PhoneNumber string `json:"phoneNumber"`
}

type userAgentPayload struct {
	AppVersion json.Number `json:"appVersion"`
	UAString   string      `json:"uaString"`
}

type userPayload struct {
	ID int64 `json:"id"`
}

type locationPayload struct {
	Coords struct {
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
		Locality string  `json:"locality"`
		Landmark string  `json:"landmark"`
	} `json:"coords"`
}

// Login opens a visible browser window and waits for the user to sign in.
func Login(ctx context.Context) (*config.Session, error) {
	tmpDir, err := os.MkdirTemp("", "blinkcli-chrome-")
	if err != nil {
		return nil, err
	}
	// Best-effort cleanup; we keep the profile for the session duration only.
	defer os.RemoveAll(tmpDir)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.UserDataDir(filepath.Clean(tmpDir)),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	if err := chromedp.Run(browserCtx,
		network.Enable(),
		chromedp.Navigate(blinkitURL),
	); err != nil {
		return nil, err
	}

	fmt.Println("A browser window is open. Please log in to Blinkit and select an address.")
	fmt.Println("Waiting for login to complete...")

	deadline := time.Now().Add(loginTimeout)
	for {
		if time.Now().After(deadline) {
			return nil, errors.New("login timed out")
		}

		session, ok, err := tryReadSession(browserCtx)
		if err != nil {
			return nil, err
		}
		if ok {
			session.UpdatedAt = time.Now()
			return session, nil
		}

		time.Sleep(loginPollDelay)
	}
}

func tryReadSession(ctx context.Context) (*config.Session, bool, error) {
	var authRaw string
	var authKey string
	var deviceID string
	var locationRaw string
	var userAgentRaw string
	var userRaw string
	var sessionID string
	var browserUA string
	var appVersionRaw string
	var rnBundleRaw string
	var cookieRaw string

	err := chromedp.Run(ctx,
		chromedp.Evaluate(`(() => { const v = localStorage.getItem("auth"); return v === null ? "" : v; })()`, &authRaw),
		chromedp.Evaluate(`(() => { const v = localStorage.getItem("authKey"); return v === null ? "" : v; })()`, &authKey),
		chromedp.Evaluate(`(() => { const v = localStorage.getItem("deviceId"); return v === null ? "" : v; })()`, &deviceID),
		chromedp.Evaluate(`(() => { const v = localStorage.getItem("location"); return v === null ? "" : v; })()`, &locationRaw),
		chromedp.Evaluate(`(() => { const v = localStorage.getItem("useragent"); return v === null ? "" : v; })()`, &userAgentRaw),
		chromedp.Evaluate(`(() => { const v = localStorage.getItem("user"); return v === null ? "" : v; })()`, &userRaw),
		chromedp.Evaluate(`(() => { const v = sessionStorage.getItem("sessionId"); return v === null ? "" : v; })()`, &sessionID),
		chromedp.Evaluate(`navigator.userAgent`, &browserUA),
		chromedp.Evaluate(`(() => window.__APP_VERSION__ || window.app_version || "")()`, &appVersionRaw),
		chromedp.Evaluate(`(() => window.__RN_BUNDLE_VERSION__ || window.rn_bundle_version || "")()`, &rnBundleRaw),
		chromedp.Evaluate(`document.cookie`, &cookieRaw),
	)
	if err != nil {
		return nil, false, err
	}

	authRaw = strings.TrimSpace(authRaw)
	if authRaw == "" {
		return nil, false, nil
	}

	var auth authPayload
	if err := json.Unmarshal([]byte(authRaw), &auth); err != nil {
		return nil, false, err
	}
	if auth.AccessToken == "" {
		return nil, false, nil
	}

	session := &config.Session{
		AccessToken: auth.AccessToken,
		AuthKey:     strings.TrimSpace(authKey),
		DeviceID:    strings.TrimSpace(deviceID),
		SessionID:   strings.TrimSpace(sessionID),
		Phone:       auth.PhoneNumber,
		Cookies:     map[string]string{},
	}

	if locationRaw != "" {
		var loc locationPayload
		if err := json.Unmarshal([]byte(locationRaw), &loc); err == nil {
			session.Lat = loc.Coords.Lat
			session.Lon = loc.Coords.Lon
			session.Locality = strings.TrimSpace(loc.Coords.Locality)
			session.Landmark = strings.TrimSpace(loc.Coords.Landmark)
		}
	}

	if userRaw != "" {
		var user userPayload
		if err := json.Unmarshal([]byte(userRaw), &user); err == nil && user.ID != 0 {
			session.UserID = fmt.Sprintf("%d", user.ID)
		}
	}

	if userAgentRaw != "" {
		var ua userAgentPayload
		if err := json.Unmarshal([]byte(userAgentRaw), &ua); err == nil {
			if ua.AppVersion != "" {
				session.WebAppVersion = ua.AppVersion.String()
				if session.AppVersion == "" {
					session.AppVersion = ua.AppVersion.String()
				}
			}
			if ua.UAString != "" {
				session.UserAgent = ua.UAString
			}
		}
	}
	if session.UserAgent == "" && browserUA != "" {
		session.UserAgent = browserUA
	}
	if session.AppVersion == "" && appVersionRaw != "" {
		session.AppVersion = appVersionRaw
	}
	if session.RNBundleVersion == "" && rnBundleRaw != "" {
		session.RNBundleVersion = rnBundleRaw
	}

	if cookies, err := network.GetCookies().WithURLs([]string{blinkitURL}).Do(ctx); err == nil {
		for _, c := range cookies {
			if strings.Contains(c.Domain, "blinkit.com") {
				session.Cookies[c.Name] = c.Value
			}
		}
	}
	if len(session.Cookies) == 0 {
		if cookies, err := storage.GetCookies().Do(ctx); err == nil {
			for _, c := range cookies {
				if strings.Contains(c.Domain, "blinkit.com") {
					session.Cookies[c.Name] = c.Value
				}
			}
		}
	}
	if cookieRaw != "" {
		for _, part := range strings.Split(cookieRaw, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name, value, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			session.Cookies[name] = strings.TrimSpace(value)
		}
	}

	if session.Lat == 0 || session.Lon == 0 {
		if lat, ok := session.Cookies["gr_1_lat"]; ok {
			if val, err := strconv.ParseFloat(lat, 64); err == nil {
				session.Lat = val
			}
		}
		if lon, ok := session.Cookies["gr_1_lon"]; ok {
			if val, err := strconv.ParseFloat(lon, 64); err == nil {
				session.Lon = val
			}
		}
	}

	config.PopulateDerivedCookies(session)

	if session.AccessToken == "" || session.AuthKey == "" || session.DeviceID == "" || session.SessionID == "" {
		// Still treat as logged in if access token exists, but caller might need to refresh.
	}

	if session.AuthKey == "" && len(session.Cookies) > 0 {
		if key, err := fetchAuthKey(session); err == nil {
			session.AuthKey = key
		}
	}

	if session.RNBundleVersion == "" {
		session.RNBundleVersion = session.WebAppVersion
	}
	if session.WebAppVersion == "" {
		session.WebAppVersion = session.AppVersion
	}
	if session.AppVersion == "" {
		session.AppVersion = session.WebAppVersion
	}

	return session, true, nil
}

// Status returns a human-readable status line and whether a session exists.
func Status(cfg *config.Config) (string, bool) {
	if cfg == nil || cfg.Session == nil || cfg.Session.AccessToken == "" {
		return "Not logged in.", false
	}
	phone := cfg.Session.Phone
	if phone == "" {
		phone = "(phone unknown)"
	}
	return fmt.Sprintf("Logged in as %s. Last updated %s.", phone, cfg.Session.UpdatedAt.Format(time.RFC3339)), true
}

func fetchAuthKey(session *config.Session) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://blinkit.com/v2/accounts/auth_key/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("app_client", "consumer_web")
	req.Header.Set("platform", "desktop_web")
	req.Header.Set("origin", blinkitOrigin)
	req.Header.Set("referer", blinkitURL)
	ua := session.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	req.Header.Set("user-agent", ua)
	if len(session.Cookies) > 0 {
		req.Header.Set("Cookie", cookieHeader(session.Cookies))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("auth_key request failed: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var payload struct {
		Success bool   `json:"success"`
		AuthKey string `json:"auth_key"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if !payload.Success || payload.AuthKey == "" {
		return "", errors.New("auth_key missing in response")
	}
	return payload.AuthKey, nil
}

func cookieHeader(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for k, v := range cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, "; ")
}
