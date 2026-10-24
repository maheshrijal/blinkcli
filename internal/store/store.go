package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"blinkcli/internal/blink"
	"blinkcli/internal/config"
)

// Store persists orders to disk.
type Store struct {
	Path string
}

func New() (*Store, error) {
	path, err := config.OrdersPath()
	if err != nil {
		return nil, err
	}
	return &Store{Path: path}, nil
}

func (s *Store) Load() ([]blink.Order, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []blink.Order{}, nil
		}
		return nil, err
	}
	var orders []blink.Order
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (s *Store) Save(orders []blink.Order) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(orders, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, data, 0o600)
}

// MergeOrders merges incoming orders into existing ones, returning the merged list and new count.
func MergeOrders(existing, incoming []blink.Order) ([]blink.Order, int) {
	seen := map[string]blink.Order{}
	unknownExisting := 0
	for _, order := range existing {
		key := orderKey(order)
		if key == "" {
			key = fmt.Sprintf("unknown-existing-%d", unknownExisting)
			unknownExisting++
		}
		seen[key] = order
	}

	newCount := 0
	unknownIncoming := 0
	for _, order := range incoming {
		key := orderKey(order)
		if key == "" {
			key = fmt.Sprintf("unknown-incoming-%d", unknownIncoming)
			unknownIncoming++
		}
		prev, ok := seen[key]
		if !ok {
			newCount++
		}
		seen[key] = mergeOrder(prev, order)
	}

	merged := make([]blink.Order, 0, len(seen))
	for _, order := range seen {
		merged = append(merged, order)
	}
	// Sort by date desc when possible.
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Date.IsZero() {
			return false
		}
		if merged[j].Date.IsZero() {
			return true
		}
		return merged[i].Date.After(merged[j].Date)
	})
	return merged, newCount
}

// mergeOrder prefers fresh fields but keeps stored ones the fresh parse left empty,
// so re-syncs repair stale data without a parse failure wiping good data.
func mergeOrder(prev, next blink.Order) blink.Order {
	if next.CartID == "" {
		next.CartID = prev.CartID
	}
	if next.Status == "" {
		next.Status = prev.Status
	}
	if next.Title == "" {
		next.Title = prev.Title
	}
	if next.AmountRupees == 0 {
		next.AmountRupees = prev.AmountRupees
	}
	if next.Date.IsZero() {
		next.Date = prev.Date
	}
	if next.RawDate == "" {
		next.RawDate = prev.RawDate
	}
	if len(next.Items) == 0 {
		next.Items = prev.Items
	}
	return next
}

func orderKey(order blink.Order) string {
	if order.ID != "" {
		return "id:" + order.ID
	}
	if order.CartID != "" {
		return "cart:" + order.CartID
	}
	// RawDate is excluded: it drifts ("Today, ..." becomes "24 Oct, ...").
	if !order.Date.IsZero() || order.AmountRupees != 0 || order.Title != "" || len(order.Items) > 0 {
		return strings.Join([]string{
			"fallback",
			order.Date.Format("2006-01-02 15:04"),
			strconvAmount(order.AmountRupees),
			order.Title,
			strings.Join(order.Items, "|"),
		}, ":")
	}
	return ""
}

func strconvAmount(amount int) string {
	return fmt.Sprintf("%d", amount)
}
