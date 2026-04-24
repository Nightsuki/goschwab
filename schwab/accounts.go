package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ListLinkedAccounts returns the account-number / hash pairs for all accounts
// linked to the authenticated user. These hashes are required for all
// subsequent account-specific API calls.
//
// Endpoint: GET /trader/v1/accounts/accountNumbers
func (c *Client) ListLinkedAccounts(ctx context.Context) ([]LinkedAccount, error) {
	var out []LinkedAccount
	if err := c.do(ctx, http.MethodGet, "/trader/v1/accounts/accountNumbers", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAllAccounts returns all brokerage accounts for the authenticated user.
//
// fields may be "" (no additional data) or "positions" (include position
// detail). Any other value is rejected client-side.
//
// Endpoint: GET /trader/v1/accounts/
func (c *Client) GetAllAccounts(ctx context.Context, fields string) ([]Account, error) {
	if fields != "" && fields != "positions" {
		return nil, fmt.Errorf("schwab: GetAllAccounts: fields must be \"\" or \"positions\", got %q", fields)
	}
	pb := newParamBuilder()
	pb.addString("fields", fields)
	var raw []json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/trader/v1/accounts/", pb.values(), nil, &raw); err != nil {
		return nil, err
	}
	accounts := make([]Account, len(raw))
	for i, r := range raw {
		a := Account{Raw: r}
		// Best-effort decode of the known top-level fields.
		if err := json.Unmarshal(r, &a); err != nil {
			return nil, fmt.Errorf("schwab: GetAllAccounts: unmarshal account[%d]: %w", i, err)
		}
		a.Raw = r
		accounts[i] = a
	}
	return accounts, nil
}

// GetAccount returns a single brokerage account identified by accountHash.
//
// fields may be "" (no additional data) or "positions" (include position
// detail). Any other value is rejected client-side.
//
// Endpoint: GET /trader/v1/accounts/{accountHash}
func (c *Client) GetAccount(ctx context.Context, accountHash, fields string) (*Account, error) {
	if fields != "" && fields != "positions" {
		return nil, fmt.Errorf("schwab: GetAccount: fields must be \"\" or \"positions\", got %q", fields)
	}
	if accountHash == "" {
		return nil, fmt.Errorf("schwab: GetAccount: accountHash must not be empty")
	}
	pb := newParamBuilder()
	pb.addString("fields", fields)
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/trader/v1/accounts/"+accountHash, pb.values(), nil, &raw); err != nil {
		return nil, err
	}
	a := &Account{Raw: raw}
	if err := json.Unmarshal(raw, a); err != nil {
		return nil, fmt.Errorf("schwab: GetAccount: unmarshal: %w", err)
	}
	a.Raw = raw
	return a, nil
}
