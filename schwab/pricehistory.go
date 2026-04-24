package schwab

import (
	"context"
	"fmt"
	"time"
)

// PriceHistoryRequest holds all parameters for GET /marketdata/v1/pricehistory.
// Zero-value fields are omitted from the query string.
type PriceHistoryRequest struct {
	// Symbol is the ticker symbol (required).
	Symbol string
	// PeriodType is the type of period: day, month, year, or ytd.
	PeriodType string
	// Period is the number of periods to return.
	Period *int
	// FrequencyType is the type of frequency: minute, daily, weekly, monthly.
	FrequencyType string
	// Frequency is the number of frequencyType units per candle.
	Frequency *int
	// StartDate filters to bars on or after this time (epoch-ms). Zero = omitted.
	StartDate time.Time
	// EndDate filters to bars on or before this time (epoch-ms). Zero = omitted.
	EndDate time.Time
	// NeedExtendedHoursData, when true, includes pre/post market bars.
	NeedExtendedHoursData *bool
	// NeedPreviousClose, when true, includes the previous close price.
	NeedPreviousClose *bool
}

// GetPriceHistory fetches OHLCV candles for the given request parameters.
// StartDate and EndDate are sent as epoch milliseconds per the Schwab spec.
//
// API: GET /marketdata/v1/pricehistory
func (c *Client) GetPriceHistory(ctx context.Context, req PriceHistoryRequest) (*PriceHistory, error) {
	if req.Symbol == "" {
		return nil, fmt.Errorf("schwab: GetPriceHistory: Symbol must not be empty")
	}

	p := newParamBuilder()
	p.addString("symbol", req.Symbol)
	p.addString("periodType", req.PeriodType)
	p.addIntPtr("period", req.Period)
	p.addString("frequencyType", req.FrequencyType)
	p.addIntPtr("frequency", req.Frequency)
	p.addTimeEpochMS("startDate", req.StartDate)
	p.addTimeEpochMS("endDate", req.EndDate)
	p.addBoolPtr("needExtendedHoursData", req.NeedExtendedHoursData)
	p.addBoolPtr("needPreviousClose", req.NeedPreviousClose)

	var out PriceHistory
	if err := c.do(ctx, "GET", "/marketdata/v1/pricehistory", p.values(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
