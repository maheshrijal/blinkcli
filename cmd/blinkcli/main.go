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

func syncCmd(args []string) {
	flags := flag.NewFlagSet("sync", flag.ExitOnError)
	maxPages := flags.Int("pages", 1, "max pages to fetch")
	pageSize := flags.Int("page-size", 0, "page size if supported by the API")
	sleepMs := flags.Int("sleep-ms", 350, "sleep between pages (ms)")
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

	client := blink.NewClient(cfg.Session)
	ctx := context.Background()

	pages := *maxPages
	if pages < 1 {
		pages = 1
	}

	merged := existing
	for page := 1; page <= pages; page++ {
		orders, err := client.OrderHistory(ctx, page, *pageSize)
		if err != nil {
			fatal(err)
		}
		updated, newCount := store.MergeOrders(merged, orders)
		fmt.Printf("Page %d/%d: fetched %d orders, new %d\n", page, pages, len(orders), newCount)
		merged = updated

		if len(orders) == 0 {
			break
		}
		if page < pages {
			time.Sleep(time.Duration(*sleepMs) * time.Millisecond)
		}
	}

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
