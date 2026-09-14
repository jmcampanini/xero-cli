package xero

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// All walks a paginated endpoint and concatenates its sole top-level resource array.
// Metadata comes from page one, with pagination updated to describe the combined body.
func All(ctx context.Context, request Request, do func(context.Context, Request) (int, http.Header, []byte, error)) (int, http.Header, []byte, error) {
	query := make(url.Values)
	for key, values := range request.Query {
		query[key] = append([]string(nil), values...)
	}
	query.Set("pageSize", "100")
	request.Query = query
	var combined map[string]json.RawMessage
	var items []json.RawMessage
	resource := ""
	pageCount := 1
	status := 0
	var headers http.Header
	for page := 1; page <= pageCount; page++ {
		query.Set("page", strconv.Itoa(page))
		code, responseHeaders, body, err := do(ctx, request)
		if err != nil {
			return code, responseHeaders, body, err
		}
		status, headers = code, responseHeaders
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(body, &envelope); err != nil {
			return status, headers, nil, apperr.New("invalid_argument", "--all requires a JSON object with pagination and one resource array")
		}
		var pagination struct {
			Page      int
			PageSize  int
			PageCount int
			ItemCount int
		}
		if err := json.Unmarshal(envelope["pagination"], &pagination); err != nil || pagination.Page != page || pagination.PageCount < page {
			return status, headers, nil, apperr.New("invalid_argument", "--all requires valid pagination with page and pageCount")
		}
		key := ""
		for field, value := range envelope {
			if len(value) > 0 && value[0] == '[' {
				if key != "" {
					return status, headers, nil, apperr.New("invalid_argument", "--all requires a single top-level resource array")
				}
				key = field
			}
		}
		if key == "" || resource != "" && resource != key {
			return status, headers, nil, apperr.New("invalid_argument", "--all requires the same single resource array on each page")
		}
		if page == 1 {
			combined = envelope
			pageCount = pagination.PageCount
			resource = key
		} else if pagination.PageCount != pageCount {
			return status, headers, nil, apperr.New("api", "pagination changed during retrieval; retry the request")
		}
		var batch []json.RawMessage
		if err := json.Unmarshal(envelope[key], &batch); err != nil {
			return status, headers, nil, apperr.New("api", "decode paginated resource: %v", err)
		}
		items = append(items, batch...)
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return status, headers, nil, err
	}
	combined[resource] = encoded
	pagination, err := json.Marshal(map[string]int{"page": 1, "pageSize": len(items), "pageCount": 1, "itemCount": len(items)})
	if err != nil {
		return status, headers, nil, err
	}
	combined["pagination"] = pagination
	body, err := json.Marshal(combined)
	return status, headers, body, err
}
