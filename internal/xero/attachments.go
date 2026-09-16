package xero

import (
	"context"
	"net/http"
	"net/url"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// Attachments lists attachment metadata on a bank transaction or manual journal.
func (c *Client) Attachments(ctx context.Context, resource, id string) ([]Record, error) {
	if resource != "BankTransactions" && resource != "ManualJournals" {
		return nil, apperr.New("invalid_argument", "attachments are not supported on %q", resource)
	}
	status, _, body, err := c.Do(ctx, http.MethodGet, resource+"/"+url.PathEscape(id)+"/Attachments", nil, nil)
	if err != nil {
		if (status == http.StatusUnauthorized || status == http.StatusForbidden) && c.lacksAttachmentScope() {
			return nil, apperr.New("forbidden", "attachments require accounting.attachments.read or accounting.attachments; update the Custom Connection in the developer portal")
		}
		return nil, err
	}
	return decodeResource[Record](body, "Attachments")
}

// lacksAttachmentScope reports whether a decodable cached token omits attachment access.
func (c *Client) lacksAttachmentScope() bool {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token == nil {
		return false
	}
	scopes, _, err := tokenClaims(c.token.AccessToken)
	return err == nil && !Capabilities(scopes)["attachments"]
}

// Attachment resolves the exact file name and requests bytes with its listed MIME type.
func (c *Client) Attachment(ctx context.Context, resource, id, name string) ([]byte, error) {
	items, err := c.Attachments(ctx, resource, id)
	if err != nil {
		return nil, err
	}
	var matches []Record
	for _, item := range items {
		if item.Text("FileName") == name {
			matches = append(matches, item)
		}
	}
	item, err := requireOne(matches, "attachment "+name+" on "+id)
	if err != nil {
		return nil, err
	}
	mime := item.Text("MimeType")
	if mime == "" {
		return nil, apperr.New("api", "attachment %q has no MIME type", name)
	}
	_, _, body, err := c.DoRequest(ctx, Request{Method: http.MethodGet,
		Path:    resource + "/" + url.PathEscape(id) + "/Attachments/" + url.PathEscape(name),
		Headers: http.Header{"Accept": {mime}},
	})
	return body, err
}
