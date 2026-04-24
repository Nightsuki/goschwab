package schwab

import (
	"context"
	"fmt"
	"net/url"
)

// GetInstruments searches for instruments matching the given symbols and
// projection. projection must be one of: symbol-search, symbol-regex,
// desc-search, desc-regex, search, or fundamental.
//
// API: GET /marketdata/v1/instruments
func (c *Client) GetInstruments(ctx context.Context, symbols []string, projection string) (*Instruments, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("schwab: GetInstruments: symbols must not be empty")
	}
	if projection == "" {
		return nil, fmt.Errorf("schwab: GetInstruments: projection must not be empty")
	}

	p := newParamBuilder()
	p.addString("symbol", formatCSV(symbols))
	p.addString("projection", projection)

	var out Instruments
	if err := c.do(ctx, "GET", "/marketdata/v1/instruments", p.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetInstrumentByCUSIP returns a single instrument record by its CUSIP.
//
// API: GET /marketdata/v1/instruments/{cusip}
func (c *Client) GetInstrumentByCUSIP(ctx context.Context, cusip string) (*Instrument, error) {
	if cusip == "" {
		return nil, fmt.Errorf("schwab: GetInstrumentByCUSIP: cusip must not be empty")
	}

	path := "/marketdata/v1/instruments/" + url.PathEscape(cusip)

	var out Instrument
	if err := c.do(ctx, "GET", path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
