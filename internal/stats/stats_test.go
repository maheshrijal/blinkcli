package stats

import (
	"strings"
	"testing"
	"time"

	"blinkcli/internal/blink"
)

func TestBuildSummaryExcludesCancelled(t *testing.T) {
	d := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	orders := []blink.Order{
		{ID: "1", Status: "DELIVERED", AmountRupees: 130, Date: d},
		{ID: "2", Status: "CANCELLED", AmountRupees: 600, Date: d},
	}
	s := BuildSummary(orders)
	if s.TotalOrders != 1 || s.TotalAmount != 130 || s.CancelledOrders != 1 {
		t.Fatalf("got orders=%d amount=%d cancelled=%d, want 1 130 1", s.TotalOrders, s.TotalAmount, s.CancelledOrders)
	}
	if len(s.Yearly) != 1 || s.Yearly[0].Count != 1 || s.Yearly[0].Amount != 130 {
		t.Fatalf("yearly bucket includes cancelled: %+v", s.Yearly)
	}
	if out := FormatSummary(s); !strings.Contains(out, "Cancelled: 1 orders (excluded)") {
		t.Fatalf("missing cancelled line:\n%s", out)
	}
}
