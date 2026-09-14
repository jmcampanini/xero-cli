package xero

import (
	"context"
	"encoding/json"
	"net/http"
)

// Organisation contains the identity, locale and financial settings used by the CLI.
type Organisation struct {
	OrganisationID        string
	Name                  string
	LegalName             string
	ShortCode             string
	CountryCode           string
	BaseCurrency          string
	Timezone              string
	OrganisationType      string
	FinancialYearEndDay   int
	FinancialYearEndMonth int
	SalesTaxBasis         string
	DefaultSalesTax       string
	DefaultPurchasesTax   string
	IsDemoCompany         bool
	raw                   json.RawMessage
}

// UnmarshalJSON retains the complete organisation object.
func (o *Organisation) UnmarshalJSON(raw []byte) error {
	type fields Organisation
	if err := json.Unmarshal(raw, (*fields)(o)); err != nil {
		return err
	}
	o.raw = append(o.raw[:0], raw...)
	return nil
}

// MarshalJSON preserves source fields and normalizes dates.
func (o Organisation) MarshalJSON() ([]byte, error) {
	if o.raw != nil {
		return NormalizeDates(o.raw)
	}
	type fields Organisation
	return json.Marshal(fields(o))
}

// Organisation fetches the effective organisation's Accounting API record.
func (c *Client) Organisation(ctx context.Context) (Organisation, error) {
	_, _, body, err := c.Do(ctx, http.MethodGet, "Organisation", nil, nil)
	if err != nil {
		return Organisation{}, err
	}
	items, err := decodeResource[Organisation](body, "Organisations")
	if err != nil {
		return Organisation{}, err
	}
	return requireOne(items, "organisation")
}
