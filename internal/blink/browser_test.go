package blink

import "testing"

func TestDesktopUserAgent(t *testing.T) {
	in := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/155.0.0.0 Safari/537.36"
	want := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36"
	if got := desktopUserAgent(in); got != want {
		t.Fatalf("desktopUserAgent() = %q, want %q", got, want)
	}
}

func TestOrderHistoryBody(t *testing.T) {
	if b, _ := orderHistoryBody(1, 0); b != nil {
		t.Fatalf("default call should send empty body, got %s", b)
	}
	if b, _ := orderHistoryBody(2, 20); string(b) != `{"page":2,"page_size":20}` {
		t.Fatalf("unexpected body %s", b)
	}
}
