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
			]
		}
	}`)

	orders, err := ParseOrderHistory(payload, time.Date(2025, 12, 20, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
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
