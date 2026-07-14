package api

import (
	"context"
	"net/url"
)

// SearchPlaces 调用 GET /api/places/search；未配置地理编码源时后端返回 501，
// 由调用方按 CodeFeatureUnavailable 处理。
func (c *Client) SearchPlaces(ctx context.Context, query string) ([]PlaceResult, error) {
	var out PlacesSearchResponse
	q := url.Values{"q": {query}}
	if err := c.Get(ctx, "/api/places/search", q, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}
