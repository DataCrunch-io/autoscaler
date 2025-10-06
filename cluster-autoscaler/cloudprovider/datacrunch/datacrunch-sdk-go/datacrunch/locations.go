package datacrunch

import "context"

type LocationService struct {
	client *Client
}

// Get retrieves all available locations
func (s *LocationService) Get(ctx context.Context) ([]Location, error) {
	locations, _, err := getRequest[[]Location](ctx, s.client, "/locations")
	if err != nil {
		return nil, err
	}
	return locations, nil
}
