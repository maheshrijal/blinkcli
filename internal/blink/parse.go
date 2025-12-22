package blink

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type orderHistoryResponse struct {
	IsSuccess bool `json:"is_success"`
	Response  struct {
		Snippets []snippet `json:"snippets"`
	} `json:"response"`
}

type snippet struct {
	WidgetType string          `json:"widget_type"`
	Data       json.RawMessage `json:"data"`
	Tracking   *trackingInfo   `json:"tracking"`
}

type trackingInfo struct {
	CommonAttributes struct {
		OrderID     string `json:"order_id"`
		OrderStatus string `json:"order_status"`
		Deeplink    string `json:"deeplink"`
	} `json:"common_attributes"`
}

type containerData struct {
	Items []snippet `json:"items"`
}

type headerData struct {
	Title struct {
		Text string `json:"text"`
	} `json:"title"`
	LeftUnderlinedSubtitle struct {
		Text string `json:"text"`
	} `json:"left_underlined_subtitle"`
	Subtitle struct {
		Text string `json:"text"`
	} `json:"subtitle"`
}

type horizontalListData struct {
	HorizontalItemList []struct {
		Data struct {
			Image struct {
				AccessibilityText struct {
					Text string `json:"text"`
				} `json:"accessibility_text"`
			} `json:"image"`
		} `json:"data"`
	} `json:"horizontal_item_list"`
}

// Order represents a parsed order from order_history.
type Order struct {
	ID           string    `json:"id"`
	CartID       string    `json:"cart_id,omitempty"`
	Status       string    `json:"status,omitempty"`
	Title        string    `json:"title,omitempty"`
	AmountRupees int       `json:"amount_rupees,omitempty"`
	Date         time.Time `json:"date"`
	RawDate      string    `json:"raw_date,omitempty"`
	Items        []string  `json:"items,omitempty"`
}

// OrderCount captures the /v1/order_count response.
type OrderCount struct {
	Delivered int `json:"delivered"`
	Live      int `json:"live"`
	Cancelled int `json:"cancelled"`
}

// ParseOrderHistory extracts orders from the order_history response.
func ParseOrderHistory(body []byte, now time.Time) ([]Order, error) {
	var resp orderHistoryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if !resp.IsSuccess {
		return nil, errors.New("order_history response not successful")
	}

	orders := make([]Order, 0)
	for _, sn := range resp.Response.Snippets {
		if sn.WidgetType != "order_history_container_vr" {
			continue
		}
		order, ok := parseOrderContainer(sn, now)
		if ok {
			orders = append(orders, order)
		}
	}
	return orders, nil
}

func parseOrderContainer(sn snippet, now time.Time) (Order, bool) {
	var data containerData
	if err := json.Unmarshal(sn.Data, &data); err != nil {
		return Order{}, false
	}

	order := Order{}
	if sn.Tracking != nil {
		order.ID = sn.Tracking.CommonAttributes.OrderID
		order.Status = sn.Tracking.CommonAttributes.OrderStatus
		if order.CartID == "" {
			order.ID, order.CartID = parseDeeplink(sn.Tracking.CommonAttributes.Deeplink, order.ID)
		}
	}

	for _, item := range data.Items {
		switch item.WidgetType {
		case "image_text_vr_type_header":
			var header headerData
			if err := json.Unmarshal(item.Data, &header); err == nil {
				order.Title = header.Title.Text
				order.RawDate = header.Subtitle.Text
				if parsed, err := ParseDate(header.Subtitle.Text, now); err == nil {
					order.Date = parsed
				}
				if amt, err := ParseAmountRupees(header.LeftUnderlinedSubtitle.Text); err == nil {
					order.AmountRupees = amt
				}
			}
		case "horizontal_list":
			items := parseHorizontalList(item)
			if len(items) > 0 {
				order.Items = append(order.Items, items...)
			}
		}
	}

	if order.ID == "" && order.RawDate == "" && order.AmountRupees == 0 {
		return Order{}, false
	}
	return order, true
}

func parseHorizontalList(sn snippet) []string {
	var list horizontalListData
	if err := json.Unmarshal(sn.Data, &list); err != nil {
		return nil
	}
	items := make([]string, 0, len(list.HorizontalItemList))
	for _, entry := range list.HorizontalItemList {
		name := strings.TrimSpace(entry.Data.Image.AccessibilityText.Text)
		if name != "" {
			items = append(items, name)
		}
	}
	return items
}

func parseDeeplink(raw, fallbackOrderID string) (string, string) {
	if raw == "" {
		return fallbackOrderID, ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fallbackOrderID, ""
	}
	q := u.Query()
	orderID := q.Get("order_id")
	cartID := q.Get("cart_id")
	if orderID == "" {
		orderID = fallbackOrderID
	}
	return orderID, cartID
}

var dateRe = regexp.MustCompile(`^(\d{1,2})\s([A-Za-z]{3})(?:,)?\s*(\d{4})?,?\s*(\d{1,2}:\d{2}\s?[ap]m)$`)

// ParseDate parses Blinkit order date strings like "19 Oct, 7:56 pm".
func ParseDate(input string, now time.Time) (time.Time, error) {
	match := dateRe.FindStringSubmatch(strings.TrimSpace(input))
	if len(match) == 0 {
		return time.Time{}, errors.New("unrecognized date format")
	}
	day, _ := strconv.Atoi(match[1])
	monthStr := match[2]
	yearStr := match[3]
	timeStr := strings.TrimSpace(match[4])

	monthTime, err := time.Parse("Jan", monthStr)
	if err != nil {
		return time.Time{}, err
	}
	year := now.Year()
	yearExplicit := false
	if yearStr != "" {
		if parsedYear, err := strconv.Atoi(yearStr); err == nil {
			year = parsedYear
			yearExplicit = true
		}
	}

	parsedTime, err := time.Parse("3:04 pm", strings.ToLower(timeStr))
	if err != nil {
		return time.Time{}, err
	}

	loc := now.Location()
	parsed := time.Date(year, monthTime.Month(), day, parsedTime.Hour(), parsedTime.Minute(), 0, 0, loc)
	if !yearExplicit && parsed.After(now.Add(24*time.Hour)) {
		parsed = parsed.AddDate(-1, 0, 0)
	}
	return parsed, nil
}

// ParseAmountRupees extracts an integer rupee amount from strings like "₹1,234".
func ParseAmountRupees(input string) (int, error) {
	clean := strings.TrimSpace(input)
	clean = strings.TrimPrefix(clean, "₹")
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return 0, errors.New("empty amount")
	}
	value, err := strconv.Atoi(clean)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func parseOrderCount(body []byte, userID string) (OrderCount, error) {
	var raw struct {
		Data map[string]struct {
			OrderTraitsRealtime struct {
				Delivered int `json:"delivered_orders"`
				Live      int `json:"live_orders"`
				Cancelled int `json:"cancelled_orders"`
			} `json:"order_traits_realtime"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return OrderCount{}, err
	}
	if len(raw.Data) == 0 {
		return OrderCount{}, errors.New("missing order_count data")
	}
	if userID != "" {
		if entry, ok := raw.Data["user:"+userID]; ok {
			return OrderCount{
				Delivered: entry.OrderTraitsRealtime.Delivered,
				Live:      entry.OrderTraitsRealtime.Live,
				Cancelled: entry.OrderTraitsRealtime.Cancelled,
			}, nil
		}
	}
	for _, entry := range raw.Data {
		return OrderCount{
			Delivered: entry.OrderTraitsRealtime.Delivered,
			Live:      entry.OrderTraitsRealtime.Live,
			Cancelled: entry.OrderTraitsRealtime.Cancelled,
		}, nil
	}
	return OrderCount{}, errors.New("missing order_count entry")
}
