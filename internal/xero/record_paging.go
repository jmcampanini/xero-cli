package xero

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// ListQuery selects a collection; Page zero fetches every page at size 100.
type ListQuery struct {
	ModifiedSince string
	Page          int
	PageSize      int
	Values        url.Values
}

// RecordList contains a complete collection or an explicitly selected page.
type RecordList struct {
	Complete  bool
	Items     []Record
	ItemCount int
	Page      int
	PageCount int
}

// Records retrieves the supported document collections with stable API ordering.
// It never returns partial records alongside an error.
func (c *Client) Records(ctx context.Context, resource string, options ListQuery) (RecordList, error) {
	idKey, err := recordID(resource)
	if err != nil {
		return RecordList{}, err
	}
	query := make(url.Values)
	for key, values := range options.Values {
		query[key] = append([]string(nil), values...)
	}
	order := "Date"
	if resource == "Contacts" {
		order = "Name"
	}
	query.Set("order", order+","+idKey)
	if resource == "BankTransactions" {
		query.Set("unitdp", "4")
	}
	request := Request{Method: http.MethodGet, Path: resource, Query: query, Headers: make(http.Header)}
	if options.ModifiedSince != "" {
		request.Headers.Set("If-Modified-Since", options.ModifiedSince)
	}
	if resource == "BankTransfers" {
		_, _, body, err := c.DoRequest(ctx, request)
		if err != nil {
			return RecordList{}, err
		}
		items, err := normalizeRecords(body, resource)
		if err != nil {
			return RecordList{}, err
		}
		return RecordList{Complete: true, Items: items, ItemCount: len(items)}, nil
	}
	page, size := options.Page, options.PageSize
	if page == 0 {
		page, size = 1, 100
	}
	if size == 0 {
		size = 100
	}
	query.Set("pageSize", strconv.Itoa(size))
	result := RecordList{Complete: options.Page == 0, Items: []Record{}}
	seen := make(map[string]bool)
	for {
		query.Set("page", strconv.Itoa(page))
		_, _, body, err := c.DoRequest(ctx, request)
		if err != nil {
			return RecordList{}, err
		}
		items, err := normalizeRecords(body, resource)
		if err != nil {
			return RecordList{}, err
		}
		var envelope struct {
			Pagination *struct{ Page, PageSize, PageCount, ItemCount int } `json:"pagination"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return RecordList{}, err
		}
		p := envelope.Pagination
		if p == nil || p.Page != page || p.PageCount < 0 || p.ItemCount < 0 {
			return RecordList{}, apperr.New("api", "%s returned invalid pagination for page %d; completeness cannot be verified", resource, page)
		}
		if p.PageSize != size {
			return RecordList{}, apperr.New("api", "%s returned pageSize %d for a requested %d; completeness cannot be verified", resource, p.PageSize, size)
		}
		if result.Page == 0 {
			result.Page, result.PageCount, result.ItemCount = page, p.PageCount, p.ItemCount
		} else if p.PageCount != result.PageCount || p.ItemCount != result.ItemCount {
			return RecordList{}, apperr.New("api", "pagination changed during retrieval; retry the request")
		}
		for _, item := range items {
			id := item.Text(idKey)
			if id == "" || seen[id] {
				return RecordList{}, apperr.New("api", "%s returned a missing or repeated ID; retry the request", resource)
			}
			seen[id] = true
		}
		result.Items = append(result.Items, items...)
		if options.Page != 0 || page >= result.PageCount {
			break
		}
		page++
	}
	if result.Complete && len(result.Items) != result.ItemCount {
		return RecordList{}, apperr.New("api", "%s returned %d of %d items; retry the request", resource, len(result.Items), result.ItemCount)
	}
	// Keep Xero's GUID tie ordering so single pages and all-page reads agree.
	// Sorting the combined set by textual GUID would move page boundaries.
	return result, nil
}
