// Package main demonstrates fetching real-time quotes via the Schwab Market
// Data API.
//
// Usage:
//
//	export SCHWAB_APP_KEY=<your-key>
//	export SCHWAB_APP_SECRET=<your-secret>
//	go run ./examples/quotes
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

	ctx := context.Background()

	c, err := schwab.NewClient(ctx, appKey, appSecret)
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	quotes, err := c.GetQuotes(ctx, []string{"AAPL", "AMD", "INTC"})
	if err != nil {
		log.Fatalf("GetQuotes: %v", err)
	}

	for symbol, q := range quotes {
		out, err := json.MarshalIndent(q, "", "  ")
		if err != nil {
			log.Fatalf("marshal quote %s: %v", symbol, err)
		}
		fmt.Printf("=== %s ===\n%s\n\n", symbol, out)
	}
}
