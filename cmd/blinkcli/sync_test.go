package main

import (
	"context"
	"errors"
	"testing"

	"blinkcli/internal/blink"
)

type fakePage struct {
	orders []blink.Order
	next   string
	err    error
}

// fakeFetcher serves pages keyed by request path and records the calls.
type fakeFetcher struct {
	pages map[string]fakePage
	calls []string
}

func (f *fakeFetcher) OrderHistory(_ context.Context, path string) ([]blink.Order, string, error) {
	f.calls = append(f.calls, path)
	p := f.pages[path]
	return p.orders, p.next, p.err
}

func TestSyncPagesFollowsNextAcrossEmptyPage(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		"":   {orders: []blink.Order{{ID: "1"}}, next: "/a"},
		"/a": {next: "/b"}, // no parseable orders, but more pages remain
		"/b": {orders: []blink.Order{{ID: "2"}}},
	}}
	merged, truncated, err := syncPages(context.Background(), f, nil, 500, 0)
	if err != nil || truncated || len(merged) != 2 {
		t.Fatalf("merged=%d truncated=%v err=%v, want 2 false nil", len(merged), truncated, err)
	}
}

func TestSyncPagesDetectsCycle(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		"":   {orders: []blink.Order{{ID: "1"}}, next: "/a"},
		"/a": {orders: []blink.Order{{ID: "2"}}, next: "/b"},
		"/b": {orders: []blink.Order{{ID: "3"}}, next: "/a"},
	}}
	_, _, err := syncPages(context.Background(), f, nil, 500, 0)
	if err == nil || len(f.calls) != 3 {
		t.Fatalf("err=%v calls=%d, want cycle error after 3 calls", err, len(f.calls))
	}
}

func TestSyncPagesKeepsProgressOnError(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		"":   {orders: []blink.Order{{ID: "1"}}, next: "/a"},
		"/a": {err: errors.New("boom")},
	}}
	merged, _, err := syncPages(context.Background(), f, nil, 500, 0)
	if err == nil || len(merged) != 1 {
		t.Fatalf("merged=%d err=%v, want 1 order and an error", len(merged), err)
	}
}

func TestSyncPagesStopsAtLimit(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		"": {orders: []blink.Order{{ID: "1"}}, next: "/a"},
	}}
	_, truncated, err := syncPages(context.Background(), f, nil, 1, 0)
	if err != nil || !truncated || len(f.calls) != 1 {
		t.Fatalf("truncated=%v err=%v calls=%d, want true nil 1", truncated, err, len(f.calls))
	}
}
