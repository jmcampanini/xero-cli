package reporting

import (
	"github.com/jmcampanini/xero-cli/internal/xero"
)

// Filters records the resolved report filters.
type Filters struct {
	Tracking []xero.TrackingFilter `json:"tracking"`
}

// By describes report breakdown columns.
type By struct {
	Category string `json:"category"`
	Note     string `json:"note,omitempty"`
}

// Output preserves report rows and the context used to request them.
type Output struct {
	Basis    string           `json:"basis,omitempty"`
	By       *By              `json:"by"`
	Columns  []string         `json:"columns"`
	Currency string           `json:"currency"`
	Date     string           `json:"date,omitempty"`
	Filters  *Filters         `json:"filters"`
	From     string           `json:"from,omitempty"`
	Org      xero.Identity    `json:"org"`
	Report   string           `json:"report"`
	Rows     []xero.ReportRow `json:"rows"`
	To       string           `json:"to,omitempty"`
}
