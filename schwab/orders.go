package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

// OrderListRequest encapsulates the query parameters for listing orders.
// From and To are required (ISO-8601 millis+Z). MaxResults defaults to 3000
// when zero. Status filters by order lifecycle state (e.g. FILLED, WORKING).
type OrderListRequest struct {
	// From is the start of the time window (required).
	From time.Time
	// To is the end of the time window (required).
	To time.Time
	// MaxResults caps the number of orders returned; 0 means use default (3000).
	MaxResults int
	// Status filters orders by lifecycle state (e.g. "FILLED", "WORKING").
	Status string
}

// orderListParams converts an OrderListRequest into url.Values.
func orderListParams(req OrderListRequest) url.Values {
	maxResults := req.MaxResults
	if maxResults == 0 {
		maxResults = 3000
	}
	pb := newParamBuilder()
	pb.addTimeISO("fromEnteredTime", req.From)
	pb.addTimeISO("toEnteredTime", req.To)
	pb.addInt("maxResults", maxResults)
	pb.addString("status", req.Status)
	return pb.values()
}

// ListAccountOrders returns orders for a single account.
//
// accountHash is the encrypted account hash from ListLinkedAccounts. From and
// To in req are required.
//
// Endpoint: GET /trader/v1/accounts/{accountHash}/orders
func (c *Client) ListAccountOrders(ctx context.Context, accountHash string, req OrderListRequest) ([]Order, error) {
	if accountHash == "" {
		return nil, fmt.Errorf("schwab: ListAccountOrders: accountHash must not be empty")
	}
	if req.From.IsZero() || req.To.IsZero() {
		return nil, fmt.Errorf("schwab: ListAccountOrders: From and To are required")
	}
	var out []Order
	p := "/trader/v1/accounts/" + accountHash + "/orders"
	if err := c.do(ctx, http.MethodGet, p, orderListParams(req), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListOrders returns orders across all linked accounts.
//
// From and To in req are required.
//
// Endpoint: GET /trader/v1/orders
func (c *Client) ListOrders(ctx context.Context, req OrderListRequest) ([]Order, error) {
	if req.From.IsZero() || req.To.IsZero() {
		return nil, fmt.Errorf("schwab: ListOrders: From and To are required")
	}
	var out []Order
	if err := c.do(ctx, http.MethodGet, "/trader/v1/orders", orderListParams(req), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PlaceOrder submits a new order for the given account. On success the
// server responds with 201 Created and a Location header containing the new
// order URL; this function extracts and returns the order ID from the last
// path segment of that URL.
//
// If the order is filled immediately the server may omit the Location header;
// in that case orderID is "" and err is nil.
//
// Endpoint: POST /trader/v1/accounts/{accountHash}/orders
func (c *Client) PlaceOrder(ctx context.Context, accountHash string, order *Order) (orderID string, err error) {
	if accountHash == "" {
		return "", fmt.Errorf("schwab: PlaceOrder: accountHash must not be empty")
	}
	if order == nil {
		return "", fmt.Errorf("schwab: PlaceOrder: order must not be nil")
	}
	p := "/trader/v1/accounts/" + accountHash + "/orders"
	resp, err := c.doRaw(ctx, http.MethodPost, p, nil, order)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	// Drain body (usually empty for 201).
	_, _ = io.Copy(io.Discard, resp.Body)
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", nil
	}
	return path.Base(loc), nil
}

// PreviewOrder previews an order without submitting it. Returns the server's
// cost, validation, and strategy analysis.
//
// Endpoint: POST /trader/v1/accounts/{accountHash}/previewOrder
func (c *Client) PreviewOrder(ctx context.Context, accountHash string, order *Order) (*OrderPreview, error) {
	if accountHash == "" {
		return nil, fmt.Errorf("schwab: PreviewOrder: accountHash must not be empty")
	}
	if order == nil {
		return nil, fmt.Errorf("schwab: PreviewOrder: order must not be nil")
	}
	p := "/trader/v1/accounts/" + accountHash + "/previewOrder"
	var out OrderPreview
	if err := c.do(ctx, http.MethodPost, p, nil, order, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOrder retrieves a single order by its ID.
//
// Endpoint: GET /trader/v1/accounts/{accountHash}/orders/{orderID}
func (c *Client) GetOrder(ctx context.Context, accountHash, orderID string) (*Order, error) {
	if accountHash == "" {
		return nil, fmt.Errorf("schwab: GetOrder: accountHash must not be empty")
	}
	if orderID == "" {
		return nil, fmt.Errorf("schwab: GetOrder: orderID must not be empty")
	}
	p := "/trader/v1/accounts/" + accountHash + "/orders/" + orderID
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, p, nil, nil, &raw); err != nil {
		return nil, err
	}
	o := &Order{Raw: raw}
	if err := json.Unmarshal(raw, o); err != nil {
		return nil, fmt.Errorf("schwab: GetOrder: unmarshal: %w", err)
	}
	o.Raw = raw
	return o, nil
}

// CancelOrder cancels an open order.
//
// Endpoint: DELETE /trader/v1/accounts/{accountHash}/orders/{orderID}
func (c *Client) CancelOrder(ctx context.Context, accountHash, orderID string) error {
	if accountHash == "" {
		return fmt.Errorf("schwab: CancelOrder: accountHash must not be empty")
	}
	if orderID == "" {
		return fmt.Errorf("schwab: CancelOrder: orderID must not be empty")
	}
	p := "/trader/v1/accounts/" + accountHash + "/orders/" + orderID
	return c.do(ctx, http.MethodDelete, p, nil, nil, nil)
}

// ReplaceOrder replaces an existing order with a new one. On success the
// server responds with 201 Created and a Location header containing the
// replacement order URL; this function extracts the new order ID from the last
// path segment.
//
// If the Location header is absent, newOrderID is "" and err is nil.
//
// Endpoint: PUT /trader/v1/accounts/{accountHash}/orders/{orderID}
func (c *Client) ReplaceOrder(ctx context.Context, accountHash, orderID string, order *Order) (newOrderID string, err error) {
	if accountHash == "" {
		return "", fmt.Errorf("schwab: ReplaceOrder: accountHash must not be empty")
	}
	if orderID == "" {
		return "", fmt.Errorf("schwab: ReplaceOrder: orderID must not be empty")
	}
	if order == nil {
		return "", fmt.Errorf("schwab: ReplaceOrder: order must not be nil")
	}
	p := "/trader/v1/accounts/" + accountHash + "/orders/" + orderID
	resp, err := c.doRaw(ctx, http.MethodPut, p, nil, order)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", nil
	}
	return path.Base(loc), nil
}
