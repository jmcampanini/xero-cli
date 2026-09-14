package xero

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// TrackingCategory contains a category and all returned options, including archived options.
type TrackingCategory struct {
	TrackingCategoryID string
	Name               string
	Status             string
	Options            []TrackingOption
	raw                json.RawMessage
}

// TrackingOption is one named option in a tracking category.
type TrackingOption struct {
	TrackingOptionID string
	Name             string
	Status           string
}

// UnmarshalJSON retains all fields returned by Xero.
func (t *TrackingCategory) UnmarshalJSON(raw []byte) error {
	type fields TrackingCategory
	if err := json.Unmarshal(raw, (*fields)(t)); err != nil {
		return err
	}
	t.raw = append(t.raw[:0], raw...)
	return nil
}

// MarshalJSON preserves the complete source object.
func (t TrackingCategory) MarshalJSON() ([]byte, error) {
	if t.raw != nil {
		return NormalizeDates(t.raw)
	}
	type fields TrackingCategory
	return json.Marshal(fields(t))
}

// TrackingCategories retrieves all categories, including archived categories.
func (c *Client) TrackingCategories(ctx context.Context) ([]TrackingCategory, error) {
	_, _, body, err := c.Do(ctx, http.MethodGet, "TrackingCategories", url.Values{"includeArchived": {"true"}}, nil)
	if err != nil {
		return nil, err
	}
	return decodeResource[TrackingCategory](body, "TrackingCategories")
}
