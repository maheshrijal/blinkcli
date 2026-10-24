package store

import (
	"path/filepath"
	"testing"
	"time"

	"blinkcli/internal/blink"
)

func TestStoreSaveLoad(t *testing.T) {
	dir := t.TempDir()
	st := &Store{Path: filepath.Join(dir, "orders.json")}

	orders := []blink.Order{
		{ID: "1", AmountRupees: 123, Date: time.Date(2025, 10, 19, 19, 56, 0, 0, time.UTC), Items: []string{"Item A"}},
		{ID: "2", AmountRupees: 456, Date: time.Date(2025, 11, 2, 11, 10, 0, 0, time.UTC), Items: []string{"Item B"}},
	}

	if err := st.Save(orders); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := st.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != len(orders) {
		t.Fatalf("expected %d orders, got %d", len(orders), len(loaded))
	}
	if loaded[0].ID == "" || loaded[1].ID == "" {
		t.Fatalf("expected ids to persist, got %+v", loaded)
	}
}

func TestMergeOrdersPrefersIncoming(t *testing.T) {
	existing := []blink.Order{{ID: "1", RawDate: "Today, 10:54 am"}}
	fixed := time.Date(2026, 10, 24, 10, 54, 0, 0, time.UTC)
	incoming := []blink.Order{{ID: "1", RawDate: "24 Oct, 10:54 am", Date: fixed}, {ID: "2"}}

	merged, newCount := MergeOrders(existing, incoming)
	if newCount != 1 || len(merged) != 2 {
		t.Fatalf("newCount=%d len=%d, want 1 and 2", newCount, len(merged))
	}
	for _, o := range merged {
		if o.ID == "1" && !o.Date.Equal(fixed) {
			t.Fatalf("stale record not replaced: %+v", o)
		}
	}
}
