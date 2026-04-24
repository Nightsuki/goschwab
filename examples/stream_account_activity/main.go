// Package main demonstrates streaming account activity events (order fills,
// cancels, etc.) via the Schwab streaming (WebSocket) API. It subscribes to
// the ACCT_ACTIVITY service and prints raw JSON frames for up to 5 minutes.
//
// Usage:
//
//	export SCHWAB_APP_KEY=<your-key>
//	export SCHWAB_APP_SECRET=<your-secret>
//	go run ./examples/stream_account_activity
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Nightsuki/goschwab/schwab"
	"github.com/Nightsuki/goschwab/schwab/stream"
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

	s := stream.New(c, stream.WithHandler(func(msg []byte) {
		fmt.Println(string(msg))
	}))

	if err := s.Start(ctx); err != nil {
		log.Fatalf("Start: %v", err)
	}

	if err := s.AccountActivity(ctx, stream.CmdSubs); err != nil {
		log.Fatalf("AccountActivity: %v", err)
	}

	fmt.Println("Streaming account activity for up to 5 minutes (Ctrl+C to stop)...")
	select {
	case <-time.After(5 * time.Minute):
	case <-ctx.Done():
	}

	if err := s.Stop(ctx, true); err != nil {
		log.Printf("Stop: %v", err)
	}
}
