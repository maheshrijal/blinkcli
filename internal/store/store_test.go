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

func TestMergeOrdersKeepsGoodFieldsOnParseFailure(t *testing.T) {
	good := time.Date(2026, 10, 9, 9, 32, 0, 0, time.UTC)
	existing := []blink.Order{{ID: "1", Date: good, AmountRupees: 228, Items: []string{"Bread"}}}
	incoming := []blink.Order{{ID: "1", Status: "DELIVERED"}} // date/amount/items failed to parse

	merged, _ := MergeOrders(existing, incoming)
	o := merged[0]
	if !o.Date.Equal(good) || o.AmountRupees != 228 || len(o.Items) != 1 || o.Status != "DELIVERED" {
		t.Fatalf("good fields lost or fresh field dropped: %+v", o)
	}
}

func TestMergeOrdersFallbackKeyIgnoresRawDateDrift(t *testing.T) {
	d := time.Date(2026, 10, 24, 10, 54, 0, 0, time.UTC)
	existing := []blink.Order{{RawDate: "Today, 10:54 am", Date: d, AmountRupees: 50}}
	incoming := []blink.Order{{RawDate: "24 Oct, 10:54 am", Date: d, AmountRupees: 50}}

	merged, newCount := MergeOrders(existing, incoming)
	if newCount != 0 || len(merged) != 1 {
		t.Fatalf("newCount=%d len=%d, want 0 and 1", newCount, len(merged))
	}
}

// At year rollover "19 Oct, 7:56 pm" becomes "19 Oct 2025" (midnight); keep the time.
func TestMergeOrdersKeepsTimeAtYearRollover(t *testing.T) {
	precise := time.Date(2025, 10, 19, 19, 56, 0, 0, time.UTC)
	midnight := time.Date(2025, 10, 19, 0, 0, 0, 0, time.UTC)
	merged, _ := MergeOrders(
		[]blink.Order{{ID: "1", Date: precise}},
		[]blink.Order{{ID: "1", Date: midnight, RawDate: "19 Oct 2025"}},
	)
	if !merged[0].Date.Equal(precise) || merged[0].RawDate != "19 Oct 2025" {
		t.Fatalf("got %+v, want precise time kept and fresh RawDate", merged[0])
	}
}

func TestMergeOrdersFallbackKeyStableAtYearRollover(t *testing.T) {
	precise := time.Date(2025, 10, 19, 19, 56, 0, 0, time.UTC)
	midnight := time.Date(2025, 10, 19, 0, 0, 0, 0, time.UTC)
	merged, newCount := MergeOrders(
		[]blink.Order{{Date: precise, AmountRupees: 50}},
		[]blink.Order{{Date: midnight, AmountRupees: 50}},
	)
	if newCount != 0 || len(merged) != 1 {
		t.Fatalf("newCount=%d len=%d, want 0 and 1", newCount, len(merged))
	}
}
