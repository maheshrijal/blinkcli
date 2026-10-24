package blink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"blinkcli/internal/config"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	// Title of Cloudflare's interstitial challenge page.
	challengeTitle   = "Just a moment"
	challengeTimeout = 20 * time.Second
)

// fetchJS runs inside the blinkit.com page so the request uses Chrome's own
// network stack (TLS fingerprint, Cloudflare cookies) instead of Go's.
const fetchJS = `(async (path, headers) => {
  const r = await fetch(path, {method: "POST", headers, credentials: "include"});
  return JSON.stringify({status: r.status, statusText: r.statusText, body: await r.text()});
})`

// BrowserClient fetches order history through a headless Chrome tab.
type BrowserClient struct {
	Session *config.Session
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewBrowserClient launches headless Chrome and loads blinkit.com.
// Call Close when done.
func NewBrowserClient(ctx context.Context, session *config.Session) (*BrowserClient, error) {
	if session == nil {
		return nil, errors.New("missing session")
	}
	profile, err := os.MkdirTemp("", "blinkcli-sync-")
	if err != nil {
		return nil, err
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.UserDataDir(profile))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	b := &BrowserClient{
		Session: session,
		ctx:     tabCtx,
		cancel: func() {
			cancelTab()
			cancelAlloc()
			_ = os.RemoveAll(profile)
		},
	}
	if err := b.open(); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}

func (b *BrowserClient) open() error {
	var ua string
	if err := chromedp.Run(b.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		_, _, _, ua, _, err = browser.GetVersion().Do(ctx)
		return err
	})); err != nil {
		return fmt.Errorf("start chrome (is Chrome installed?): %w", err)
	}
	// Cloudflare challenges the default "HeadlessChrome" user agent.
	if err := chromedp.Run(b.ctx,
		emulation.SetUserAgentOverride(desktopUserAgent(ua)),
		chromedp.Navigate(baseURL+"/"),
	); err != nil {
		return err
	}
	deadline := time.Now().Add(challengeTimeout)
	for {
		var title string
		if err := chromedp.Run(b.ctx, chromedp.Title(&title)); err != nil {
			return err
		}
		if !strings.Contains(title, challengeTitle) {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("blocked by Cloudflare challenge on blinkit.com")
		}
		time.Sleep(time.Second)
	}
}

// OrderHistory fetches one page from inside the page; see Client.OrderHistory.
func (b *BrowserClient) OrderHistory(_ context.Context, path string) ([]Order, string, error) {
	path, err := PagePath(path)
	if err != nil {
		return nil, "", err
	}
	args, err := json.Marshal([]any{path, apiHeaders(b.Session)})
	if err != nil {
		return nil, "", err
	}
	expr := fetchJS + "(..." + string(args) + ")"

	var raw string
	if err := chromedp.Run(b.ctx, chromedp.Evaluate(expr, &raw, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return nil, "", err
	}
	var res struct {
		Status     int    `json:"status"`
		StatusText string `json:"statusText"`
		Body       string `json:"body"`
	}
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return nil, "", err
	}
	if res.Status == 401 || res.Status == 403 {
		// Cloudflare already passed in open(), so this is the API rejecting the session.
		return nil, "", fmt.Errorf("order_history request failed: %d (session expired? run 'blinkcli auth login')", res.Status)
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, "", fmt.Errorf("order_history request failed: %s", strings.TrimSpace(fmt.Sprintf("%d %s", res.Status, res.StatusText)))
	}
	return ParseOrderHistory([]byte(res.Body), time.Now())
}

// Close shuts down Chrome and removes the temporary profile.
func (b *BrowserClient) Close() {
	b.cancel()
}

// desktopUserAgent strips the headless marker from Chrome's user agent.
func desktopUserAgent(ua string) string {
	return strings.Replace(ua, "HeadlessChrome/", "Chrome/", 1)
}
