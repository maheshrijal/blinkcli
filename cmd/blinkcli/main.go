package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"blinkcli/internal/auth"
	"blinkcli/internal/blink"
	"blinkcli/internal/config"
	"blinkcli/internal/format"
	"blinkcli/internal/stats"
	"blinkcli/internal/store"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "auth":
		authCmd(os.Args[2:])
	case "version":
		fmt.Println(version)
	case "sync":
		syncCmd(os.Args[2:])
	case "orders":
		ordersCmd()
	case "stats":
		statsCmd()
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("blinkcli - unofficial Blinkit CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  blinkcli auth login")
	fmt.Println("  blinkcli auth status")
	fmt.Println("  blinkcli auth logout")
	fmt.Println("  blinkcli version")
	fmt.Println("  blinkcli sync")
	fmt.Println("  blinkcli orders")
	fmt.Println("  blinkcli stats")
}

func authCmd(args []string) {
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	switch args[0] {
	case "login":
		session, err := auth.Login(context.Background())
		if err != nil {
			fatal(err)
		}
		cfg := &config.Config{Session: session}
		if err := config.Save(cfg); err != nil {
			fatal(err)
		}
		fmt.Println("Login captured and saved.")
	case "status":
		cfg, err := config.Load()
		if err != nil {
			fatal(err)
		}
		msg, _ := auth.Status(cfg)
		fmt.Println(msg)
	case "logout":
		if err := config.Clear(); err != nil {
			fatal(err)
		}
		fmt.Println("Logged out (local session cleared).")
	default:
		usage()
		os.Exit(1)
	}
}

const maxSyncPages = 500

// orderFetcher is implemented by both the HTTP and headless-Chrome clients.
type orderFetcher interface {
	OrderHistory(ctx context.Context, path string) ([]blink.Order, string, error)
}

func syncCmd(args []string) {
	flags := flag.NewFlagSet("sync", flag.ExitOnError)
	maxPages := flags.Int("pages", 0, "max pages to fetch (0 = all)")
	sleepMs := flags.Int("sleep-ms", 350, "sleep between pages (ms)")
	useHTTP := flags.Bool("http", false, "call the API directly instead of via headless Chrome")
	_ = flags.Int("page-size", 0, "deprecated: ignored by the API")
	_ = flags.Parse(args)

	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	if cfg.Session == nil || cfg.Session.AccessToken == "" {
		fatal(fmt.Errorf("not logged in; run 'blinkcli auth login'"))
	}

	st, err := store.New()
	if err != nil {
		fatal(err)
	}
	existing, err := st.Load()
	if err != nil {
		fatal(err)
	}

	ctx := context.Background()
	var client orderFetcher = blink.NewClient(cfg.Session)
	closeClient := func() {}
	if !*useHTTP {
		bc, err := blink.NewBrowserClient(ctx, cfg.Session)
		if err != nil {
			fatal(err)
		}
		// Closed explicitly: fatal's os.Exit would skip a defer and orphan Chrome.
		closeClient = bc.Close
		client = bc
	}

	// Hard cap guards against a pagination loop.
	pages := maxSyncPages
	if *maxPages > maxSyncPages {
		fmt.Fprintf(os.Stderr, "Warning: --pages capped at %d.\n", maxSyncPages)
	} else if *maxPages > 0 {
		pages = *maxPages
	}

	merged, truncated, syncErr := syncPages(ctx, client, existing, pages, time.Duration(*sleepMs)*time.Millisecond)
	closeClient()

	// Save progress even on error so a later page failure doesn't discard earlier pages.
	if err := st.Save(merged); err != nil {
		fatal(err)
	}
	if syncErr != nil {
		fatal(fmt.Errorf("%w (saved %d orders fetched so far)", syncErr, len(merged)))
	}
	if truncated {
		fmt.Printf("Stopped at the --pages limit (%d); more history remains.\n", pages)
	}
	fmt.Printf("Sync complete. Stored %d orders.\n", len(merged))
}

// syncPages follows next_url until exhausted or pages is reached. It returns the
// orders merged so far even on error, and whether it stopped at the page limit.
func syncPages(ctx context.Context, client orderFetcher, existing []blink.Order, pages int, sleep time.Duration) ([]blink.Order, bool, error) {
	merged := existing
	path := ""
	visited := map[string]bool{}
	for page := 1; ; page++ {
		visited[path] = true
		orders, next, err := client.OrderHistory(ctx, path)
		if err != nil {
			return merged, false, fmt.Errorf("page %d: %w", page, err)
		}
		updated, newCount := store.MergeOrders(merged, orders)
		fmt.Printf("Page %d: fetched %d orders, new %d\n", page, len(orders), newCount)
		merged = updated

		// next_url alone decides the end; a page may hold no parseable orders.
		if next == "" {
			return merged, false, nil
		}
		if visited[next] {
			return merged, false, fmt.Errorf("pagination loop at page %d", page)
		}
		if page >= pages {
			return merged, true, nil
		}
		path = next
		time.Sleep(sleep)
	}
}

func ordersCmd() {
	st, err := store.New()
	if err != nil {
		fatal(err)
	}
	orders, err := st.Load()
	if err != nil {
		fatal(err)
	}
	if len(orders) == 0 {
		fmt.Println("No orders stored yet. Run 'blinkcli sync'.")
		return
	}
	fmt.Println(format.OrdersTable(orders))
}

func statsCmd() {
	st, err := store.New()
	if err != nil {
		fatal(err)
	}
	orders, err := st.Load()
	if err != nil {
		fatal(err)
	}
	if len(orders) == 0 {
		fmt.Println("No orders stored yet. Run 'blinkcli sync'.")
		return
	}
	summary := stats.BuildSummary(orders)
	fmt.Println(stats.FormatSummary(summary))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
