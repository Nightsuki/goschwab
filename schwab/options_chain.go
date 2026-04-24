package schwab

import (
	"context"
	"fmt"
	"time"
)

// ContractType restricts an option chain request to calls, puts, or both.
type ContractType string

const (
	// ContractTypeCall requests only call options.
	ContractTypeCall ContractType = "CALL"
	// ContractTypePut requests only put options.
	ContractTypePut ContractType = "PUT"
	// ContractTypeAll requests both calls and puts.
	ContractTypeAll ContractType = "ALL"
)

// OptionChainRequest holds all parameters for GET /marketdata/v1/chains.
// Zero-value fields are omitted from the query string.
type OptionChainRequest struct {
	// Symbol is the underlying symbol (required).
	Symbol string
	// ContractType filters by CALL, PUT, or ALL (default ALL when zero).
	ContractType ContractType
	// StrikeCount limits the number of strikes around the money.
	StrikeCount *int
	// IncludeUnderlyingQuote, when true, embeds the underlying quote.
	IncludeUnderlyingQuote *bool
	// Strategy is the chain strategy (e.g. SINGLE, ANALYTICAL, COVERED, ...).
	Strategy string
	// Interval is the strike interval for spread strategies.
	Interval *float64
	// Strike filters to a specific strike price.
	Strike *float64
	// Range is the moneyness filter (ITM, NTM, OTM, SAK, SBK, SNK, ALL).
	Range string
	// FromDate is the earliest expiration date (YYYY-MM-DD). Zero = omitted.
	FromDate time.Time
	// ToDate is the latest expiration date (YYYY-MM-DD). Zero = omitted.
	ToDate time.Time
	// Volatility overrides the volatility input for ANALYTICAL strategy.
	Volatility *float64
	// UnderlyingPrice overrides the underlying price for ANALYTICAL strategy.
	UnderlyingPrice *float64
	// InterestRate overrides the risk-free rate for ANALYTICAL strategy.
	InterestRate *float64
	// DaysToExpiration overrides the DTE for ANALYTICAL strategy.
	DaysToExpiration *int
	// ExpMonth filters by expiration month (e.g. "JAN", "ALL").
	ExpMonth string
	// OptionType filters by option type (e.g. "S" for standard, "NS", "ALL").
	OptionType string
	// Entitlement is the client entitlement (PP, NP, PN).
	Entitlement string
}

// GetOptionChain fetches an option chain for the given request parameters.
//
// API: GET /marketdata/v1/chains
func (c *Client) GetOptionChain(ctx context.Context, req OptionChainRequest) (*OptionChain, error) {
	if req.Symbol == "" {
		return nil, fmt.Errorf("schwab: GetOptionChain: Symbol must not be empty")
	}

	p := newParamBuilder()
	p.addString("symbol", req.Symbol)
	p.addString("contractType", string(req.ContractType))
	p.addIntPtr("strikeCount", req.StrikeCount)
	p.addBoolPtr("includeUnderlyingQuote", req.IncludeUnderlyingQuote)
	p.addString("strategy", req.Strategy)
	p.addFloatPtr("interval", req.Interval)
	p.addFloatPtr("strike", req.Strike)
	p.addString("range", req.Range)
	p.addTimeYMD("fromDate", req.FromDate)
	p.addTimeYMD("toDate", req.ToDate)
	p.addFloatPtr("volatility", req.Volatility)
	p.addFloatPtr("underlyingPrice", req.UnderlyingPrice)
	p.addFloatPtr("interestRate", req.InterestRate)
	p.addIntPtr("daysToExpiration", req.DaysToExpiration)
	p.addString("expMonth", req.ExpMonth)
	p.addString("optionType", req.OptionType)
	p.addString("entitlement", req.Entitlement)

	var out OptionChain
	if err := c.do(ctx, "GET", "/marketdata/v1/chains", p.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOptionExpirationChain returns the list of option expirations for a symbol.
//
// API: GET /marketdata/v1/expirationchain
func (c *Client) GetOptionExpirationChain(ctx context.Context, symbol string) (*ExpirationChain, error) {
	if symbol == "" {
		return nil, fmt.Errorf("schwab: GetOptionExpirationChain: symbol must not be empty")
	}

	p := newParamBuilder()
	p.addString("symbol", symbol)

	var out ExpirationChain
	if err := c.do(ctx, "GET", "/marketdata/v1/expirationchain", p.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
