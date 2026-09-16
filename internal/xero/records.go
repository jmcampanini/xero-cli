package xero

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// Record retains every field of a document, including fields unknown to the CLI.
// Known money fields are decimal strings; tracking retains Xero's native names.
type Record map[string]json.RawMessage

// Text returns a scalar's text without converting JSON numbers through float64.
func (r Record) Text(key string) string {
	raw := r[key]
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	if raw[0] == '"' {
		var value string
		_ = json.Unmarshal(raw, &value)
		return value
	}
	return string(raw)
}

// Object returns a nested record, or nil when absent.
func (r Record) Object(key string) Record {
	var value Record
	_ = json.Unmarshal(r[key], &value)
	return value
}

// Records returns a nested array, or nil when absent.
func (r Record) Records(key string) []Record {
	var value []Record
	_ = json.Unmarshal(r[key], &value)
	return value
}

// ScalarKeys returns scalar field names in a stable order for contact details.
func (r Record) ScalarKeys() []string {
	var keys []string
	for key, raw := range r {
		if len(raw) > 0 && raw[0] != '[' && raw[0] != '{' {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func normalizeRecords(body []byte, resource string) ([]Record, error) {
	normalized, err := NormalizeDates(body)
	if err != nil {
		return nil, apperr.New("api", "decode %s: %v", resource, err)
	}
	items, err := decodeResource[Record](normalized, resource)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if err := normalizeMoney(item); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func normalizeMoney(record Record) error {
	for key, raw := range record {
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			continue
		}
		if raw[0] == '{' {
			object := record.Object(key)
			if err := normalizeMoney(object); err != nil {
				return err
			}
			record[key], _ = json.Marshal(object)
			continue
		}
		if raw[0] == '[' {
			var values []json.RawMessage
			if err := json.Unmarshal(raw, &values); err != nil {
				return err
			}
			for i, value := range values {
				if len(value) > 0 && value[0] == '{' {
					var object Record
					if err := json.Unmarshal(value, &object); err != nil {
						return err
					}
					if err := normalizeMoney(object); err != nil {
						return err
					}
					values[i], _ = json.Marshal(object)
				}
			}
			record[key], _ = json.Marshal(values)
			continue
		}
		switch key {
		case "Amount", "SubTotal", "TotalTax", "Total", "UnitAmount", "LineAmount", "TaxAmount", "DiscountAmount", "BankAccountBalance", "AmountDue", "AmountPaid", "AmountCredited", "Outstanding", "Overdue", "DebitTotal", "CreditTotal":
			value := record.Text(key)
			if !ValidDecimal(value) {
				return apperr.New("api", "Xero returned an invalid decimal for %s", key)
			}
			record[key], _ = json.Marshal(value)
		}
	}
	return nil
}

func recordID(resource string) (string, error) {
	switch resource {
	case "BankTransactions":
		return "BankTransactionID", nil
	case "BankTransfers":
		return "BankTransferID", nil
	case "ManualJournals":
		return "ManualJournalID", nil
	case "Contacts":
		return "ContactID", nil
	default:
		return "", apperr.New("invalid_argument", "unsupported resource %q", resource)
	}
}

// Record retrieves a document by GUID with normalized dates and exact money.
func (c *Client) Record(ctx context.Context, resource, id string) (Record, error) {
	if _, err := recordID(resource); err != nil {
		return nil, err
	}
	query := make(url.Values)
	if resource == "BankTransactions" {
		query.Set("unitdp", "4")
	}
	_, _, body, err := c.Do(ctx, http.MethodGet, resource+"/"+url.PathEscape(id), query, nil)
	if err != nil {
		return nil, apperr.New(apperr.From(err).Code, "%s %s: %s", resource, id, err)
	}
	items, err := normalizeRecords(body, resource)
	if err != nil {
		return nil, err
	}
	return requireOne(items, resource+" "+id)
}
