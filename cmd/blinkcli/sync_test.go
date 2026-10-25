package main

import (
	"context"
	"errors"
	"testing"

	"blinkcli/internal/blink"
)

const (
	first = "/v1/layout/order_history"
	pageA = first + "?cursor=a"
	pageB = first + "?cursor=b"
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
		first: {orders: []blink.Order{{ID: "1"}}, next: pageA},
		pageA: {next: pageB}, // no parseable orders, but more pages remain
		pageB: {orders: []blink.Order{{ID: "2"}}},
	}}
	merged, truncated, err := syncPages(context.Background(), f, nil, 500, 0)
	if err != nil || truncated || len(merged) != 2 {
		t.Fatalf("merged=%d truncated=%v err=%v, want 2 false nil", len(merged), truncated, err)
	}
}

func TestSyncPagesDetectsCycle(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		first: {orders: []blink.Order{{ID: "1"}}, next: pageA},
		pageA: {orders: []blink.Order{{ID: "2"}}, next: pageB},
		pageB: {orders: []blink.Order{{ID: "3"}}, next: pageA},
	}}
	_, _, err := syncPages(context.Background(), f, nil, 500, 0)
	if err == nil || len(f.calls) != 3 {
		t.Fatalf("err=%v calls=%d, want cycle error after 3 calls", err, len(f.calls))
	}
}

func TestSyncPagesDetectsLoopToFirstPage(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		first: {orders: []blink.Order{{ID: "1"}}, next: first},
	}}
	_, _, err := syncPages(context.Background(), f, nil, 500, 0)
	if err == nil || len(f.calls) != 1 {
		t.Fatalf("err=%v calls=%d, want loop error after 1 call", err, len(f.calls))
	}
}

func TestSyncPagesRejectsForeignNextURL(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		first: {orders: []blink.Order{{ID: "1"}}, next: "/v1/user/logout"},
	}}
	merged, _, err := syncPages(context.Background(), f, nil, 500, 0)
	if err == nil || len(f.calls) != 1 || len(merged) != 1 {
		t.Fatalf("err=%v calls=%d merged=%d, want error, 1 call, 1 order", err, len(f.calls), len(merged))
	}
}

func TestSyncPagesLimitBeatsBadNextURL(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		first: {orders: []blink.Order{{ID: "1"}}, next: "/v1/somewhere/else"},
	}}
	_, truncated, err := syncPages(context.Background(), f, nil, 1, 0)
	if err != nil || !truncated {
		t.Fatalf("truncated=%v err=%v, want true nil", truncated, err)
	}
}

func TestSyncPagesKeepsProgressOnError(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		first: {orders: []blink.Order{{ID: "1"}}, next: pageA},
		pageA: {err: errors.New("boom")},
	}}
	merged, _, err := syncPages(context.Background(), f, nil, 500, 0)
	if err == nil || len(merged) != 1 {
		t.Fatalf("merged=%d err=%v, want 1 order and an error", len(merged), err)
	}
}

func TestSyncPagesStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &fakeFetcher{pages: map[string]fakePage{
		first: {orders: []blink.Order{{ID: "1"}}, next: pageA},
	}}
	merged, _, err := syncPages(ctx, f, nil, 500, 0)
	if !errors.Is(err, context.Canceled) || len(merged) != 1 || len(f.calls) != 1 {
		t.Fatalf("err=%v merged=%d calls=%d, want canceled, 1, 1", err, len(merged), len(f.calls))
	}
}

func TestSyncPagesStopsAtLimit(t *testing.T) {
	f := &fakeFetcher{pages: map[string]fakePage{
		first: {orders: []blink.Order{{ID: "1"}}, next: pageA},
	}}
	_, truncated, err := syncPages(context.Background(), f, nil, 1, 0)
	if err != nil || !truncated || len(f.calls) != 1 {
		t.Fatalf("truncated=%v err=%v calls=%d, want true nil 1", truncated, err, len(f.calls))
	}
}
