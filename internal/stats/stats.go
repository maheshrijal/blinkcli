package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"blinkcli/internal/blink"
)

type Summary struct {
	TotalOrders int
	TotalAmount int
	Monthly     []Bucket
	Yearly      []Bucket
}

type Bucket struct {
	Label  string
	Count  int
	Amount int
}

// BuildSummary aggregates orders into monthly and yearly buckets.
func BuildSummary(orders []blink.Order) Summary {
	monthly := map[string]*Bucket{}
	yearly := map[string]*Bucket{}

	for _, order := range orders {
		if order.Date.IsZero() {
			continue
		}
		monthKey := order.Date.Format("2006-01")
		yearKey := order.Date.Format("2006")

		addBucket(monthly, monthKey, order)
		addBucket(yearly, yearKey, order)
	}

	return Summary{
		TotalOrders: len(orders),
		TotalAmount: sumAmounts(orders),
		Monthly:     sortedBuckets(monthly),
		Yearly:      sortedBuckets(yearly),
	}
}

func addBucket(store map[string]*Bucket, key string, order blink.Order) {
	bucket, ok := store[key]
	if !ok {
		bucket = &Bucket{Label: key}
		store[key] = bucket
	}
	bucket.Count++
	bucket.Amount += order.AmountRupees
}

func sumAmounts(orders []blink.Order) int {
	total := 0
	for _, order := range orders {
		total += order.AmountRupees
	}
	return total
}

func sortedBuckets(store map[string]*Bucket) []Bucket {
	buckets := make([]Bucket, 0, len(store))
	for _, b := range store {
		buckets = append(buckets, *b)
	}
	sort.Slice(buckets, func(i, j int) bool {
		// Sort by label ascending (YYYY or YYYY-MM).
		return buckets[i].Label < buckets[j].Label
	})
	return buckets
}

// FormatSummary returns a short, human-readable report.
func FormatSummary(summary Summary) string {
	lines := []string{
		fmt.Sprintf("Total: %d orders, ₹%d", summary.TotalOrders, summary.TotalAmount),
	}

	if len(summary.Yearly) > 0 {
		lines = append(lines, "Yearly:")
		for _, b := range summary.Yearly {
			lines = append(lines, fmt.Sprintf("  %s: %d orders, ₹%d", b.Label, b.Count, b.Amount))
		}
	}

	if len(summary.Monthly) > 0 {
		lines = append(lines, "Monthly:")
		for _, b := range summary.Monthly {
			lines = append(lines, fmt.Sprintf("  %s: %d orders, ₹%d", b.Label, b.Count, b.Amount))
		}
	}

	// Include timestamp to show when stats were generated.
	lines = append(lines, fmt.Sprintf("Generated at %s", time.Now().Format(time.RFC3339)))
	return strings.Join(lines, "\n")
}
