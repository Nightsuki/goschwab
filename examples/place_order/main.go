// Package main demonstrates building a market order and (optionally) placing
// it via the Schwab Trader API.
//
// By default this example only prints the constructed order JSON and does NOT
// submit it. To actually place the order, set SCHWAB_LIVE=1:
//
//	export SCHWAB_APP_KEY=<your-key>
//	export SCHWAB_APP_SECRET=<your-secret>
//	go run ./examples/place_order          # dry-run (safe)
//	SCHWAB_LIVE=1 go run ./examples/place_order  # places a real order!
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Nightsuki/goschwab/schwab"
)

func main() {
	appKey := os.Getenv("SCHWAB_APP_KEY")
	appSecret := os.Getenv("SCHWAB_APP_SECRET")
	if appKey == "" || appSecret == "" {
		log.Fatal("SCHWAB_APP_KEY and SCHWAB_APP_SECRET must be set")
	}

	live := os.Getenv("SCHWAB_LIVE") == "1"

	ctx := context.Background()

	c, err := schwab.NewClient(ctx, appKey, appSecret)
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	// Fetch linked accounts and use the first one.
	linked, err := c.ListLinkedAccounts(ctx)
	if err != nil {
		log.Fatalf("ListLinkedAccounts: %v", err)
	}
	if len(linked) == 0 {
		log.Fatal("no linked accounts found")
	}
	accountHash := linked[0].HashValue
	fmt.Printf("Using account hash: %s\n\n", accountHash)

	// Build a minimal BUY 1 AAPL market day-order using the raw JSON leg
	// format that Schwab's API expects.
	legJSON, err := json.Marshal([]map[string]interface{}{
		{
			"instruction": "BUY",
			"quantity":    1,
			"instrument": map[string]string{
				"symbol":    "AAPL",
				"assetType": "EQUITY",
			},
		},
	})
	if err != nil {
		log.Fatalf("marshal leg: %v", err)
	}

	order := &schwab.Order{
		OrderType:          "MARKET",
		Session:            "NORMAL",
		Duration:           "DAY",
		OrderStrategyType:  "SINGLE",
		OrderLegCollection: json.RawMessage(legJSON),
	}

	out, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		log.Fatalf("marshal order: %v", err)
	}
	fmt.Println("Constructed order:")
	fmt.Println(string(out))

	if !live {
		fmt.Println("\n[DRY RUN] Order was NOT submitted.")
		fmt.Println("To actually place this order, re-run with SCHWAB_LIVE=1.")
		return
	}

	// *** LIVE MODE: actually places the order ***
	orderID, err := c.PlaceOrder(ctx, accountHash, order)
	if err != nil {
		log.Fatalf("PlaceOrder: %v", err)
	}
	if orderID == "" {
		fmt.Println("Order placed (filled immediately; no order ID returned).")
	} else {
		fmt.Printf("Order placed successfully. Order ID: %s\n", orderID)
	}
}
