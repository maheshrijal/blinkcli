package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	// Write into the symlink target, if any, so renaming doesn't replace the link.
	path := s.Path
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if path, err = filepath.EvalSymlinks(path); err != nil {
			return fmt.Errorf("orders.json symlink is broken: %w", err)
		}
	}
	// Temp file (CreateTemp uses 0600) + fsync + rename: never a truncated orders.json.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".orders-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	// fsync the directory so the rename itself survives a crash.
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
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

// mergeOrder prefers fresh fields but keeps stored ones a failed parse left empty.
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
	} else if sameDay(prev.Date, next.Date) && isMidnight(next.Date) && !isMidnight(prev.Date) {
		// Older-year dates lose their time ("19 Oct 2025"); keep the stored time of day.
		p, n := prev.Date, next.Date
		next.Date = time.Date(n.Year(), n.Month(), n.Day(), p.Hour(), p.Minute(), 0, 0, n.Location())
	}
	if next.RawDate == "" {
		next.RawDate = prev.RawDate
	}
	if len(next.Items) == 0 {
		next.Items = prev.Items
	}
	return next
}

// sameDay compares wall-clock dates (legacy rows hold IST values in the local zone).
func sameDay(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func isMidnight(t time.Time) bool {
	return t.Hour() == 0 && t.Minute() == 0
}

func orderKey(order blink.Order) string {
	if order.ID != "" {
		return "id:" + order.ID
	}
	if order.CartID != "" {
		return "cart:" + order.CartID
	}
	// Best effort without an ID; RawDate drifts, so use it only if the date didn't parse.
	if !order.Date.IsZero() || order.AmountRupees != 0 || order.Title != "" || order.RawDate != "" || len(order.Items) > 0 {
		rawDate := ""
		if order.Date.IsZero() {
			rawDate = order.RawDate
		}
		return strings.Join([]string{
			"fallback",
			order.Date.Format("2006-01-02 15:04"),
			rawDate,
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
