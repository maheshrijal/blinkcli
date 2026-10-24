package blink

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	base := time.Date(2025, 12, 20, 12, 0, 0, 0, time.UTC)
	parsed, err := ParseDate("19 Oct, 7:56 pm", base)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parsed.Year() != 2025 || parsed.Month() != time.October || parsed.Day() != 19 || parsed.Hour() != 19 || parsed.Minute() != 56 {
		t.Fatalf("unexpected parsed date: %v", parsed)
	}
}

// Formats seen on a real account (2026-10-24): relative, current-year, older-year.
func TestParseDateFormats(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	now := time.Date(2026, 10, 24, 21, 0, 0, 0, ist)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"Today, 10:54 am", time.Date(2026, 10, 24, 10, 54, 0, 0, ist)},
		{"Yesterday, 9:05 pm", time.Date(2026, 10, 23, 21, 5, 0, 0, ist)},
		{"09 Oct, 9:32 am", time.Date(2026, 10, 9, 9, 32, 0, 0, ist)},
		{"04 Jan, 7:35 pm", time.Date(2026, 1, 4, 19, 35, 0, 0, ist)},
		{"19 Oct 2025", time.Date(2025, 10, 19, 0, 0, 0, 0, ist)},
		{"26 Jun 2024", time.Date(2024, 6, 26, 0, 0, 0, 0, ist)},
	}
	for _, tc := range cases {
		got, err := ParseDate(tc.in, now)
		if err != nil {
			t.Errorf("ParseDate(%q) error: %v", tc.in, err)
			continue
		}
		if !got.Equal(tc.want) {
			t.Errorf("ParseDate(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// A machine in UTC at 20:00 Oct 23 is already Oct 24 in IST, where Blinkit's "Today" lives.
func TestParseDateAnchorsToIST(t *testing.T) {
	now := time.Date(2026, 10, 23, 20, 0, 0, 0, time.UTC)
	got, err := ParseDate("Today, 12:30 am", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 24, 0, 30, 0, 0, ist)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseAmountRupees(t *testing.T) {
	amount, err := ParseAmountRupees("₹1,234")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if amount != 1234 {
		t.Fatalf("expected 1234, got %d", amount)
	}
}

func TestParseOrderHistoryItems(t *testing.T) {
	payload := []byte(`{
		"is_success": true,
		"response": {
			"snippets": [
				{
					"widget_type": "order_history_container_vr",
					"data": {
						"items": [
							{
								"widget_type": "image_text_vr_type_header",
								"data": {
									"title": {"text": "Arrived in 9 minutes"},
									"left_underlined_subtitle": {"text": "₹493"},
									"subtitle": {"text": "19 Oct, 7:56 pm"}
								}
							},
							{
								"widget_type": "horizontal_list",
								"data": {
									"horizontal_item_list": [
										{"data": {"image": {"accessibility_text": {"text": "Item A"}}}},
										{"data": {"image": {"accessibility_text": {"text": "Item B"}}}}
									]
								}
							}
						]
					},
					"tracking": {
						"common_attributes": {
							"order_id": "123",
							"order_status": "DELIVERED",
							"deeplink": "grofers://widgetized/order_details_v2?order_id=123&cart_id=999"
						}
					}
				}
			],
			"pagination": {"next_url": "/v1/layout/order_history?offset=0&limit=10&cursor=abc"}
		}
	}`)

	orders, next, err := ParseOrderHistory(payload, time.Date(2025, 12, 20, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if next != "/v1/layout/order_history?offset=0&limit=10&cursor=abc" {
		t.Fatalf("unexpected next url: %q", next)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].ID != "123" || orders[0].CartID != "999" {
		t.Fatalf("unexpected order ids: %+v", orders[0])
	}
	if len(orders[0].Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(orders[0].Items))
	}
}
