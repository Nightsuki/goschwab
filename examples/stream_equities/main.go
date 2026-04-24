// Package main demonstrates streaming level-1 equity quotes via the Schwab
// streaming (WebSocket) API. It subscribes to AMD, INTC, and $SPX and prints
// raw JSON frames for 30 seconds before stopping.
//
// Usage:
//
//	export SCHWAB_APP_KEY=<your-key>
//	export SCHWAB_APP_SECRET=<your-secret>
//	go run ./examples/stream_equities
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

	fields := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"}
	if err := s.LevelOneEquities(ctx, []string{"AMD", "INTC", "$SPX"}, fields, stream.CmdAdd); err != nil {
		log.Fatalf("LevelOneEquities: %v", err)
	}

	fmt.Println("Streaming for 30 seconds...")
	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
	}

	if err := s.Stop(ctx, true); err != nil {
		log.Printf("Stop: %v", err)
	}
}
