package schwab

import (
	"context"
	"fmt"
	"net/url"
)

// MoverSort controls the sort order for the movers response.
type MoverSort string

const (
	// MoverSortVolume sorts movers by volume.
	MoverSortVolume MoverSort = "VOLUME"
	// MoverSortTrades sorts movers by trade count.
	MoverSortTrades MoverSort = "TRADES"
	// MoverSortPercentChangeUp sorts movers by percent change (ascending).
	MoverSortPercentChangeUp MoverSort = "PERCENT_CHANGE_UP"
	// MoverSortPercentChangeDown sorts movers by percent change (descending).
	MoverSortPercentChangeDown MoverSort = "PERCENT_CHANGE_DOWN"
)

// moverConfig holds the resolved options for a movers request.
type moverConfig struct {
	sort      MoverSort
	frequency *int
}

// MoverOption configures a GetMovers call.
type MoverOption func(*moverConfig)

// WithMoverSort sets the sort criterion for the movers response.
func WithMoverSort(s MoverSort) MoverOption {
	return func(c *moverConfig) { c.sort = s }
}

// WithMoverFrequency sets the frequency value (valid: 0, 1, 5, 10, 30, 60).
func WithMoverFrequency(f int) MoverOption {
	return func(c *moverConfig) { c.frequency = &f }
}

// GetMovers returns the top movers for the given index symbol (e.g. "$DJI",
// "$COMPX", "$SPX", "NYSE", "NASDAQ", "OTCBB", "INDEX_ALL", "EQUITY_ALL",
// "OPTION_ALL", "OPTION_PUT", "OPTION_CALL").
//
// API: GET /marketdata/v1/movers/{symbol}
func (c *Client) GetMovers(ctx context.Context, symbol string, opts ...MoverOption) (*Movers, error) {
	if symbol == "" {
		return nil, fmt.Errorf("schwab: GetMovers: symbol must not be empty")
	}

	var cfg moverConfig
	for _, o := range opts {
		o(&cfg)
	}

	path := "/marketdata/v1/movers/" + url.PathEscape(symbol)

	p := newParamBuilder()
	p.addString("sort", string(cfg.sort))
	p.addIntPtr("frequency", cfg.frequency)

	var out Movers
	if err := c.do(ctx, "GET", path, p.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
