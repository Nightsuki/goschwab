package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TransactionsRequest encapsulates the query parameters for listing
// transactions. Start and End are required (ISO-8601 millis+Z).
type TransactionsRequest struct {
	// Start is the beginning of the time window (required).
	Start time.Time
	// End is the end of the time window (required).
	End time.Time
	// Types filters by transaction type (e.g. "TRADE", "DIVIDEND_OR_INTEREST").
	Types string
	// Symbol filters to transactions involving a specific symbol.
	Symbol string
}

// ListTransactions returns transactions for the given account within the
// time window specified by req.
//
// Endpoint: GET /trader/v1/accounts/{accountHash}/transactions
func (c *Client) ListTransactions(ctx context.Context, accountHash string, req TransactionsRequest) ([]Transaction, error) {
	if accountHash == "" {
		return nil, fmt.Errorf("schwab: ListTransactions: accountHash must not be empty")
	}
	if req.Start.IsZero() || req.End.IsZero() {
		return nil, fmt.Errorf("schwab: ListTransactions: Start and End are required")
	}
	pb := newParamBuilder()
	pb.addTimeISO("startDate", req.Start)
	pb.addTimeISO("endDate", req.End)
	pb.addString("types", req.Types)
	pb.addString("symbol", req.Symbol)
	var out []Transaction
	p := "/trader/v1/accounts/" + accountHash + "/transactions"
	if err := c.do(ctx, http.MethodGet, p, pb.values(), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTransaction retrieves a single transaction by its ID.
//
// Endpoint: GET /trader/v1/accounts/{accountHash}/transactions/{transactionID}
func (c *Client) GetTransaction(ctx context.Context, accountHash, transactionID string) (*Transaction, error) {
	if accountHash == "" {
		return nil, fmt.Errorf("schwab: GetTransaction: accountHash must not be empty")
	}
	if transactionID == "" {
		return nil, fmt.Errorf("schwab: GetTransaction: transactionID must not be empty")
	}
	p := "/trader/v1/accounts/" + accountHash + "/transactions/" + transactionID
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, p, nil, nil, &raw); err != nil {
		return nil, err
	}
	tx := &Transaction{Raw: raw}
	if err := json.Unmarshal(raw, tx); err != nil {
		return nil, fmt.Errorf("schwab: GetTransaction: unmarshal: %w", err)
	}
	tx.Raw = raw
	return tx, nil
}
