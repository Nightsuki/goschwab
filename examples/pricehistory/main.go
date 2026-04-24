// Package main demonstrates fetching OHLCV candles via the Schwab Market
// Data API.
//
// Usage:
//
//	export SCHWAB_APP_KEY=<your-key>
//	export SCHWAB_APP_SECRET=<your-secret>
//	go run ./examples/pricehistory
package main

import (
	"context"
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

	period := 5
	frequency := 5
	req := schwab.PriceHistoryRequest{
		Symbol:        "SPY",
		PeriodType:    "day",
		Period:        &period,
		FrequencyType: "minute",
		Frequency:     &frequency,
	}

	hist, err := c.GetPriceHistory(ctx, req)
	if err != nil {
		log.Fatalf("GetPriceHistory: %v", err)
	}

	fmt.Printf("Symbol:  %s\n", hist.Symbol)
	fmt.Printf("Candles: %d\n", len(hist.Candles))

	if len(hist.Candles) > 0 {
		first := hist.Candles[0]
		last := hist.Candles[len(hist.Candles)-1]
		fmt.Printf("First candle: open=%.4f high=%.4f low=%.4f close=%.4f volume=%d datetime=%d\n",
			first.Open, first.High, first.Low, first.Close, first.Volume, first.Datetime)
		fmt.Printf("Last  candle: open=%.4f high=%.4f low=%.4f close=%.4f volume=%d datetime=%d\n",
			last.Open, last.High, last.Low, last.Close, last.Volume, last.Datetime)
	}
}
