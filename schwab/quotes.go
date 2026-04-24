package schwab

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// QuoteFields controls which field groups are included in a quote response.
type QuoteFields string

const (
	// QuoteFieldsAll requests all available field groups.
	QuoteFieldsAll QuoteFields = "all"
	// QuoteFieldsQuote requests only the quote (NBBO/NFL) fields.
	QuoteFieldsQuote QuoteFields = "quote"
	// QuoteFieldsFundamental requests only the fundamental data fields.
	QuoteFieldsFundamental QuoteFields = "fundamental"
)

// quoteConfig holds the resolved options for a quote request.
type quoteConfig struct {
	fields    []QuoteFields
	indicative *bool
}

// QuoteOption configures a GetQuotes or GetQuote call.
type QuoteOption func(*quoteConfig)

// WithQuoteFields restricts the response to the specified field groups.
// Pass QuoteFieldsAll, QuoteFieldsQuote, or QuoteFieldsFundamental.
func WithQuoteFields(fields ...QuoteFields) QuoteOption {
	return func(c *quoteConfig) {
		c.fields = append(c.fields, fields...)
	}
}

// WithIndicative, when true, includes indicative (non-tradable) symbols in
// the response. Applicable to index symbols.
func WithIndicative(v bool) QuoteOption {
	return func(c *quoteConfig) { c.indicative = &v }
}

// buildQuoteQuery converts a quoteConfig into url.Values.
func buildQuoteQuery(cfg quoteConfig) url.Values {
	p := newParamBuilder()
	if len(cfg.fields) > 0 {
		parts := make([]string, len(cfg.fields))
		for i, f := range cfg.fields {
			parts[i] = strings.ToLower(string(f))
		}
		p.addString("fields", strings.Join(parts, ","))
	}
	p.addBoolPtr("indicative", cfg.indicative)
	return p.values()
}

// GetQuotes returns quotes for one or more symbols. symbols is CSV-encoded
// into a single "symbols" query parameter (e.g. "AMD,INTC").
//
// API: GET /marketdata/v1/quotes
func (c *Client) GetQuotes(ctx context.Context, symbols []string, opts ...QuoteOption) (map[string]Quote, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("schwab: GetQuotes: symbols must not be empty")
	}

	var cfg quoteConfig
	for _, o := range opts {
		o(&cfg)
	}

	p := newParamBuilder()
	p.addString("symbols", formatCSV(symbols))
	q := buildQuoteQuery(cfg)
	for k, vs := range q {
		for _, v := range vs {
			p.v.Set(k, v)
		}
	}

	var out map[string]Quote
	if err := c.do(ctx, "GET", "/marketdata/v1/quotes", p.values(), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetQuote returns the quote for a single symbol. The symbol is
// URL-path-escaped so that slash-containing symbols like "BRK/B" are safe.
//
// API: GET /marketdata/v1/{symbol}/quotes
func (c *Client) GetQuote(ctx context.Context, symbol string, opts ...QuoteOption) (*Quote, error) {
	if symbol == "" {
		return nil, fmt.Errorf("schwab: GetQuote: symbol must not be empty")
	}

	var cfg quoteConfig
	for _, o := range opts {
		o(&cfg)
	}

	path := "/marketdata/v1/" + url.PathEscape(symbol) + "/quotes"
	q := buildQuoteQuery(cfg)

	var out map[string]Quote
	if err := c.do(ctx, "GET", path, q, nil, &out); err != nil {
		return nil, err
	}
	if q2, ok := out[symbol]; ok {
		return &q2, nil
	}
	// Schwab sometimes returns an upper-cased key; do a case-insensitive scan.
	upper := strings.ToUpper(symbol)
	for k, v := range out {
		if strings.ToUpper(k) == upper {
			cp := v
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("schwab: GetQuote: symbol %q not found in response", symbol)
}
