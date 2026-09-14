package xero

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Account is a chart-of-accounts record; JSON output also preserves unknown Xero fields.
type Account struct {
	AccountID               string
	Code                    string
	Name                    string
	Type                    string
	Class                   string
	Status                  string
	TaxType                 string
	Description             string
	EnablePaymentsToAccount bool
	ShowInExpenseClaims     bool
	BankAccountNumber       string
	BankAccountType         string
	CurrencyCode            string
	ReportingCode           string
	UpdatedDateUTC          string
	raw                     json.RawMessage
}

// UnmarshalJSON retains every source field for faithful machine output.
func (a *Account) UnmarshalJSON(raw []byte) error {
	type fields Account
	if err := json.Unmarshal(raw, (*fields)(a)); err != nil {
		return err
	}
	a.raw = append(a.raw[:0], raw...)
	return nil
}

// MarshalJSON normalizes dates without dropping source fields.
func (a Account) MarshalJSON() ([]byte, error) {
	if a.raw != nil {
		return NormalizeDates(a.raw)
	}
	type fields Account
	raw, err := json.Marshal(fields(a))
	if err != nil {
		return nil, err
	}
	return NormalizeDates(raw)
}

// Accounts lists the chart, which Xero returns without pagination.
func (c *Client) Accounts(ctx context.Context, query url.Values) ([]Account, error) {
	_, _, body, err := c.Do(ctx, http.MethodGet, "Accounts", query, nil)
	if err != nil {
		return nil, err
	}
	return decodeResource[Account](body, "Accounts")
}

// Account fetches a single account by its Xero GUID.
func (c *Client) Account(ctx context.Context, id string) (Account, error) {
	_, _, body, err := c.Do(ctx, http.MethodGet, "Accounts/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return Account{}, err
	}
	items, err := decodeResource[Account](body, "Accounts")
	if err != nil {
		return Account{}, err
	}
	return requireOne(items, "account "+id)
}
