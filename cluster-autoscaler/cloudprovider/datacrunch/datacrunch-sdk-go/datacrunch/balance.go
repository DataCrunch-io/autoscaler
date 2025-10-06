package datacrunch

import "context"

type BalanceService struct {
	client *Client
}

// Get retrieves the current account balance
func (s *BalanceService) Get(ctx context.Context) (*Balance, error) {
	// Uses client's default interceptors (auth, JSON content type, error handling)
	balance, _, err := getRequest[Balance](ctx, s.client, "/balance")
	if err != nil {
		return nil, err
	}
	return &balance, nil
}
