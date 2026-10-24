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
	if p, _ := pagePath(""); p != orderHistoryPath {
		t.Fatalf("empty path should default to first page, got %q", p)
	}
	if p, err := pagePath("/v1/layout/order_history?cursor=x"); err != nil || p != "/v1/layout/order_history?cursor=x" {
		t.Fatalf("relative path rejected: %q %v", p, err)
	}
	for _, bad := range []string{"https://evil.example/x", "//evil.example/x"} {
		if _, err := pagePath(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}
