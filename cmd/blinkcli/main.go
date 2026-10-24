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
	if *maxPages > 0 && *maxPages < pages {
		pages = *maxPages
	}

	merged := existing
	path := ""
	for page := 1; page <= pages; page++ {
		orders, next, err := client.OrderHistory(ctx, path)
		if err != nil {
			closeClient()
			fatal(err)
		}
		updated, newCount := store.MergeOrders(merged, orders)
		fmt.Printf("Page %d: fetched %d orders, new %d\n", page, len(orders), newCount)
		merged = updated

		if next == "" || next == path || len(orders) == 0 {
			break
		}
		path = next
		time.Sleep(time.Duration(*sleepMs) * time.Millisecond)
	}

	closeClient()

	if err := st.Save(merged); err != nil {
		fatal(err)
	}
	fmt.Printf("Sync complete. Stored %d orders.\n", len(merged))
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
