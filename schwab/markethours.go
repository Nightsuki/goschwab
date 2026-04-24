package schwab

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// GetMarketHours returns the trading hours for one or more market types on
// the given date. markets elements should be equity, option, bond, future, or
// forex. A zero date omits the date parameter, defaulting to today.
//
// API: GET /marketdata/v1/markets
func (c *Client) GetMarketHours(ctx context.Context, markets []string, date time.Time) (*MarketHours, error) {
	if len(markets) == 0 {
		return nil, fmt.Errorf("schwab: GetMarketHours: markets must not be empty")
	}

	p := newParamBuilder()
	p.addString("markets", formatCSV(markets))
	p.addTimeYMD("date", date)

	var out MarketHours
	if err := c.do(ctx, "GET", "/marketdata/v1/markets", p.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMarketHour returns the trading hours for a single market type on the
// given date. market should be equity, option, bond, future, or forex.
// A zero date omits the date parameter, defaulting to today.
//
// API: GET /marketdata/v1/markets/{market}
func (c *Client) GetMarketHour(ctx context.Context, market string, date time.Time) (*MarketHours, error) {
	if market == "" {
		return nil, fmt.Errorf("schwab: GetMarketHour: market must not be empty")
	}

	path := "/marketdata/v1/markets/" + url.PathEscape(market)

	p := newParamBuilder()
	p.addTimeYMD("date", date)

	var out MarketHours
	if err := c.do(ctx, "GET", path, p.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
