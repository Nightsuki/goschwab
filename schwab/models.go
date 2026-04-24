package schwab

import (
	"encoding/json"
	"time"
)

// LinkedAccount is an account-number / hash pair as returned by
// GET /trader/v1/accounts/accountNumbers.
type LinkedAccount struct {
	// AccountNumber is the plaintext account number.
	AccountNumber string `json:"accountNumber"`
	// HashValue is the encrypted hash used for all subsequent account calls.
	HashValue string `json:"hashValue"`
}

// Account is a single Schwab brokerage account. Many fields are heavily
// nested and vary by account type; the Raw field preserves the full body.
type Account struct {
	// SecuritiesAccount is the core account payload.
	SecuritiesAccount json.RawMessage `json:"securitiesAccount,omitempty"`
	// AggregatedBalance is the top-level balance summary when returned.
	AggregatedBalance json.RawMessage `json:"aggregatedBalance,omitempty"`
	// Raw is the entire response body, for callers that want full fidelity.
	Raw json.RawMessage `json:"-"`
}

// Position describes a single holding within an account. Fields vary by
// security type; callers requiring detail should decode Raw themselves.
type Position struct {
	// ShortQuantity is the shorted quantity.
	ShortQuantity *float64 `json:"shortQuantity,omitempty"`
	// LongQuantity is the held quantity.
	LongQuantity *float64 `json:"longQuantity,omitempty"`
	// AveragePrice is the average cost price.
	AveragePrice *float64 `json:"averagePrice,omitempty"`
	// MarketValue is the current market value.
	MarketValue *float64 `json:"marketValue,omitempty"`
	// Instrument describes the underlying security.
	Instrument json.RawMessage `json:"instrument,omitempty"`
	// Raw preserves the full record.
	Raw json.RawMessage `json:"-"`
}

// Order is the request/response body for the Trader Orders API. The Schwab
// order schema is large and polymorphic; we preserve full fidelity via
// RawMessage fields and surface the most-used scalars.
type Order struct {
	// OrderID is the server-issued identifier (only present on responses).
	OrderID *int64 `json:"orderId,omitempty"`
	// Session is NORMAL | AM | PM | SEAMLESS.
	Session string `json:"session,omitempty"`
	// Duration is DAY | GOOD_TILL_CANCEL | FILL_OR_KILL | IMMEDIATE_OR_CANCEL.
	Duration string `json:"duration,omitempty"`
	// OrderType is LIMIT | MARKET | STOP | STOP_LIMIT | ...
	OrderType string `json:"orderType,omitempty"`
	// ComplexOrderStrategyType is NONE | COVERED | VERTICAL | ...
	ComplexOrderStrategyType string `json:"complexOrderStrategyType,omitempty"`
	// Quantity is the order quantity.
	Quantity *float64 `json:"quantity,omitempty"`
	// FilledQuantity is the quantity filled so far.
	FilledQuantity *float64 `json:"filledQuantity,omitempty"`
	// RemainingQuantity is quantity still open.
	RemainingQuantity *float64 `json:"remainingQuantity,omitempty"`
	// Price is the limit or stop price depending on OrderType.
	Price *float64 `json:"price,omitempty"`
	// OrderStrategyType is SINGLE | OCO | TRIGGER.
	OrderStrategyType string `json:"orderStrategyType,omitempty"`
	// Status is the lifecycle state (e.g. FILLED, WORKING, CANCELED).
	Status string `json:"status,omitempty"`
	// EnteredTime is the server-side creation timestamp.
	EnteredTime *time.Time `json:"enteredTime,omitempty"`
	// CloseTime is the server-side close timestamp.
	CloseTime *time.Time `json:"closeTime,omitempty"`
	// AccountNumber is the plaintext account number (response only).
	AccountNumber *int64 `json:"accountNumber,omitempty"`
	// OrderLegCollection is the per-leg breakdown.
	OrderLegCollection json.RawMessage `json:"orderLegCollection,omitempty"`
	// ChildOrderStrategies for OCO / TRIGGER orders.
	ChildOrderStrategies json.RawMessage `json:"childOrderStrategies,omitempty"`
	// Raw preserves the entire payload for callers that need full fidelity.
	Raw json.RawMessage `json:"-"`
}

// OrderPreview is the response of POST /accounts/{hash}/previewOrder.
type OrderPreview struct {
	// OrderID echoed when present.
	OrderID *int64 `json:"orderId,omitempty"`
	// OrderStrategy is the preview's order-strategy block.
	OrderStrategy json.RawMessage `json:"orderStrategy,omitempty"`
	// OrderValidationResult is the validation-warnings / rejects block.
	OrderValidationResult json.RawMessage `json:"orderValidationResult,omitempty"`
	// CommissionAndFee is the cost breakdown.
	CommissionAndFee json.RawMessage `json:"commissionAndFee,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}

// Transaction is a single account transaction row.
type Transaction struct {
	// ActivityID is the transaction identifier.
	ActivityID *int64 `json:"activityId,omitempty"`
	// Time is the transaction timestamp.
	Time *time.Time `json:"time,omitempty"`
	// Type is TRADE, DIVIDEND_OR_INTEREST, ACH_DISBURSEMENT, etc.
	Type string `json:"type,omitempty"`
	// Status is the clearance status.
	Status string `json:"status,omitempty"`
	// SubAccount is the source sub-account.
	SubAccount string `json:"subAccount,omitempty"`
	// NetAmount is the cash delta.
	NetAmount *float64 `json:"netAmount,omitempty"`
	// TransferItems itemizes the transaction legs.
	TransferItems json.RawMessage `json:"transferItems,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}

// Quote is the unified quote block for a single symbol. The Schwab payload
// varies by asset class; Quote/Reference/Fundamental/Regular are kept raw.
type Quote struct {
	// AssetMainType is EQUITY | OPTION | FUTURE | FOREX | INDEX | ...
	AssetMainType string `json:"assetMainType,omitempty"`
	// AssetSubType refines AssetMainType when applicable.
	AssetSubType string `json:"assetSubType,omitempty"`
	// QuoteType is NBBO | NFL for equities.
	QuoteType string `json:"quoteType,omitempty"`
	// Realtime indicates the quote is real-time rather than delayed.
	Realtime *bool `json:"realtime,omitempty"`
	// SSID is the Schwab-internal instrument identifier.
	SSID *int64 `json:"ssid,omitempty"`
	// Symbol is the requested symbol.
	Symbol string `json:"symbol,omitempty"`
	// Quote holds the NBBO/NFL quote fields.
	Quote json.RawMessage `json:"quote,omitempty"`
	// Reference holds the static reference data.
	Reference json.RawMessage `json:"reference,omitempty"`
	// Regular holds the regular-market snapshot.
	Regular json.RawMessage `json:"regular,omitempty"`
	// Fundamental holds the fundamental data block (when requested).
	Fundamental json.RawMessage `json:"fundamental,omitempty"`
	// Extended holds the extended-hours snapshot.
	Extended json.RawMessage `json:"extended,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}

// Candle is a single OHLCV bar from the price-history endpoint.
type Candle struct {
	// Open price.
	Open float64 `json:"open"`
	// High price.
	High float64 `json:"high"`
	// Low price.
	Low float64 `json:"low"`
	// Close price.
	Close float64 `json:"close"`
	// Volume traded.
	Volume int64 `json:"volume"`
	// Datetime is the bar timestamp in epoch milliseconds.
	Datetime int64 `json:"datetime"`
}

// PriceHistory is the response of GET /marketdata/v1/pricehistory.
type PriceHistory struct {
	// Symbol is the requested symbol.
	Symbol string `json:"symbol"`
	// Empty is true when no candles were returned.
	Empty bool `json:"empty"`
	// Candles is the list of OHLCV bars.
	Candles []Candle `json:"candles"`
	// PreviousClose is the prior close price.
	PreviousClose *float64 `json:"previousClose,omitempty"`
	// PreviousCloseDate is epoch-ms of the prior close date.
	PreviousCloseDate *int64 `json:"previousCloseDate,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}

// OptionChain is the response of GET /marketdata/v1/chains.
type OptionChain struct {
	// Symbol is the underlying symbol.
	Symbol string `json:"symbol"`
	// Status is SUCCESS / FAILED.
	Status string `json:"status"`
	// Strategy echoes the request strategy.
	Strategy string `json:"strategy"`
	// Interval echoes the request interval.
	Interval *float64 `json:"interval,omitempty"`
	// IsDelayed indicates whether the quote stream is delayed.
	IsDelayed *bool `json:"isDelayed,omitempty"`
	// IsIndex indicates whether the underlying is an index.
	IsIndex *bool `json:"isIndex,omitempty"`
	// DaysToExpiration is the default DTE for the chain.
	DaysToExpiration *float64 `json:"daysToExpiration,omitempty"`
	// InterestRate echoed from the request.
	InterestRate *float64 `json:"interestRate,omitempty"`
	// UnderlyingPrice is the spot price used for Greeks.
	UnderlyingPrice *float64 `json:"underlyingPrice,omitempty"`
	// Volatility echoed from the request.
	Volatility *float64 `json:"volatility,omitempty"`
	// Underlying is the full underlying quote block.
	Underlying json.RawMessage `json:"underlying,omitempty"`
	// CallExpDateMap maps "YYYY-MM-DD:DTE" → strike → []contract.
	CallExpDateMap json.RawMessage `json:"callExpDateMap,omitempty"`
	// PutExpDateMap maps "YYYY-MM-DD:DTE" → strike → []contract.
	PutExpDateMap json.RawMessage `json:"putExpDateMap,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}

// ExpirationChain is the response of GET /marketdata/v1/expirationchain.
type ExpirationChain struct {
	// Status is SUCCESS / FAILED.
	Status string `json:"status"`
	// ExpirationList is the array of expirations.
	ExpirationList json.RawMessage `json:"expirationList,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}

// Movers is the response of GET /marketdata/v1/movers/{symbol}.
type Movers struct {
	// Screeners is the raw movers list.
	Screeners json.RawMessage `json:"screeners,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}

// MarketHours is the response of GET /marketdata/v1/markets[/{market}].
type MarketHours struct {
	// Raw preserves the entire payload; keys are market types, values contain
	// sessionHours maps and metadata.
	Raw json.RawMessage `json:"-"`
}

// MarshalJSON preserves Raw if set; otherwise marshals an empty object.
func (m MarketHours) MarshalJSON() ([]byte, error) {
	if len(m.Raw) > 0 {
		return m.Raw, nil
	}
	return []byte("{}"), nil
}

// UnmarshalJSON stashes the entire payload into Raw.
func (m *MarketHours) UnmarshalJSON(data []byte) error {
	m.Raw = append(m.Raw[:0], data...)
	return nil
}

// Instrument is a single instrument record from the Instruments API.
type Instrument struct {
	// CUSIP is the CUSIP identifier.
	CUSIP string `json:"cusip,omitempty"`
	// Symbol is the ticker symbol.
	Symbol string `json:"symbol,omitempty"`
	// Description is a human-readable name.
	Description string `json:"description,omitempty"`
	// Exchange is the listing exchange.
	Exchange string `json:"exchange,omitempty"`
	// AssetType is EQUITY | OPTION | FUTURE | MUTUAL_FUND | ETF | ...
	AssetType string `json:"assetType,omitempty"`
	// Fundamental holds the fundamental data block, when requested.
	Fundamental json.RawMessage `json:"fundamental,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}

// Instruments is the response of GET /marketdata/v1/instruments.
type Instruments struct {
	// Instruments is the list of matched instruments.
	Instruments []Instrument `json:"instruments,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}

// StreamerInfo describes the user's streamer endpoint + credentials.
type StreamerInfo struct {
	// StreamerSocketURL is the wss:// URL for the streaming service.
	StreamerSocketURL string `json:"streamerSocketUrl"`
	// SchwabClientCustomerID identifies the streaming customer.
	SchwabClientCustomerID string `json:"schwabClientCustomerId"`
	// SchwabClientCorrelID is the correlation ID used in streamer requests.
	SchwabClientCorrelID string `json:"schwabClientCorrelId"`
	// SchwabClientChannel is the LOGIN channel identifier.
	SchwabClientChannel string `json:"schwabClientChannel"`
	// SchwabClientFunctionID is the LOGIN function identifier.
	SchwabClientFunctionID string `json:"schwabClientFunctionId"`
}

// UserPreferences is the response of GET /trader/v1/userPreference.
type UserPreferences struct {
	// Accounts is the per-account preferences list.
	Accounts json.RawMessage `json:"accounts,omitempty"`
	// StreamerInfo is the array of streaming endpoints (usually length 1).
	StreamerInfo []StreamerInfo `json:"streamerInfo,omitempty"`
	// Offers holds any feature-availability flags.
	Offers json.RawMessage `json:"offers,omitempty"`
	// Raw preserves the entire payload.
	Raw json.RawMessage `json:"-"`
}
