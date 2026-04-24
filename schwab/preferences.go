package schwab

import (
	"context"
	"net/http"
)

// GetUserPreferences returns the authenticated user's trading preferences.
//
// Endpoint: GET /trader/v1/userPreference
func (c *Client) GetUserPreferences(ctx context.Context) (*UserPreferences, error) {
	var out UserPreferences
	if err := c.do(ctx, http.MethodGet, "/trader/v1/userPreference", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
