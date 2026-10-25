package blink

import "testing"

func TestDesktopUserAgent(t *testing.T) {
	in := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/155.0.0.0 Safari/537.36"
	want := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36"
	if got := desktopUserAgent(in); got != want {
		t.Fatalf("desktopUserAgent() = %q, want %q", got, want)
	}
}

func TestPagePath(t *testing.T) {
	good := map[string]string{
		"":                                  orderHistoryPath,
		"/v1/layout/order_history?cursor=x": "/v1/layout/order_history?cursor=x",
		"https://blinkit.com/v1/layout/order_history?cursor=x":     "/v1/layout/order_history?cursor=x",
		"https://www.blinkit.com/v1/layout/order_history?cursor=x": "/v1/layout/order_history?cursor=x",
		"https://blinkit.com:443/v1/layout/order_history":          "/v1/layout/order_history",
		"https://Blinkit.COM/v1/layout/order_history":              "/v1/layout/order_history",
	}
	for in, want := range good {
		if got, err := PagePath(in); err != nil || got != want {
			t.Errorf("PagePath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{
		"https://evil.example/v1/layout/order_history",
		"//evil.example/v1/layout/order_history",
		`/\evil.example/v1/layout/order_history`,
		"http://blinkit.com/v1/layout/order_history",
		"https://blinkit.com:8443/v1/layout/order_history",
		"https://blinkit.com.evil.example/v1/layout/order_history",
		"https://user@blinkit.com/v1/layout/order_history",
		"javascript:alert(1)",
		"/v1/user/logout",
		"/v1/layout/order_history/../../user/logout",
		"v1/layout/order_history",
	}
	for _, in := range bad {
		if _, err := PagePath(in); err == nil {
			t.Errorf("PagePath(%q) accepted, want rejection", in)
		}
	}
}
