package format

import (
	"fmt"
	"strings"
	"time"

	"blinkcli/internal/blink"
)

// OrdersTable renders orders in a compact, line-based format.
func OrdersTable(orders []blink.Order) string {
	lines := make([]string, 0, len(orders)+1)
	lines = append(lines, "DATE | AMOUNT | ORDER ID | ITEMS")
	for _, order := range orders {
		date := ""
		if !order.Date.IsZero() {
			date = order.Date.Format("2006-01-02 15:04")
		}
		amount := fmt.Sprintf("₹%d", order.AmountRupees)
		items := strings.Join(order.Items, ", ")
		if len(items) > 60 {
			items = items[:57] + "..."
		}
		lines = append(lines, fmt.Sprintf("%s | %s | %s | %s", date, amount, order.ID, items))
	}
	return strings.Join(lines, "\n")
}

// RelativeDate is a helper for CLI display when date is missing.
func RelativeDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}
