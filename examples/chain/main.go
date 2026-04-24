// Package main demonstrates fetching an option chain via the Schwab Market
// Data API.
//
// Usage:
//
//	export SCHWAB_APP_KEY=<your-key>
//	export SCHWAB_APP_SECRET=<your-secret>
//	go run ./examples/chain
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

	strikeCount := 10
	req := schwab.OptionChainRequest{
		Symbol:       "AAPL",
		ContractType: schwab.ContractTypeAll,
		StrikeCount:  &strikeCount,
	}

	chain, err := c.GetOptionChain(ctx, req)
	if err != nil {
		log.Fatalf("GetOptionChain: %v", err)
	}

	out, err := json.MarshalIndent(chain, "", "  ")
	if err != nil {
		log.Fatalf("marshal chain: %v", err)
	}
	fmt.Println(string(out))
}
