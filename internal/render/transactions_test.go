package render

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/xero-cli/internal/xero"
)

func TestTransactionCSVKeepsExactRowsAndSeparatesContext(t *testing.T) {
	result := xero.AccountTransactions{Account: xero.TransactionAccount{Code: "429", Name: "Supplies"}, Basis: "cash", Currency: "USD", From: "2025-07-01", To: "2025-07-31", Org: xero.Identity{Name: "Example"}, OpeningBalance: "1000.00", OpeningPeriod: "financial_year_to_date", ClosingBalance: "1012.5000", TrialBalance: "1015.00", Unexplained: "2.5000", NetMovement: "12.5000", Gap: "payments excluded", Rows: []xero.AccountTransaction{{Date: "2025-07-02", SourceType: "bank_transaction", SourceSubtype: "SPEND", SourceID: "full-source-identifier", Contact: "Supplier", Description: "Paper, pens\nextra", Reference: "ref", Debit: "12.5000", Balance: "1012.5000", Status: "AUTHORISED"}}}
	var out, diagnostics bytes.Buffer
	if err := Transactions(&out, &diagnostics, result, true, "always"); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&out).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"date", "source_type", "source_subtype", "source_id", "contact", "description", "reference", "debit", "credit", "balance", "tracking", "status"},
		{"2025-07-02", "bank_transaction", "SPEND", "full-source-identifier", "Supplier", "Paper, pens\nextra", "ref", "12.5000", "", "1012.5000", "", "AUTHORISED"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("CSV = %v, want %v", rows, want)
	}
	for _, fragment := range []string{"cash · USD", "opening balance 1,000.00", "reconstructed closing balance 1,012.5000", "unexplained 2.5000", "incomplete coverage: payments excluded"} {
		if !strings.Contains(diagnostics.String(), fragment) {
			t.Errorf("missing %q in %q", fragment, diagnostics.String())
		}
	}
	if strings.Contains(diagnostics.String(), "\x1b") {
		t.Error("CSV context contains color")
	}
	out.Reset()
	diagnostics.Reset()
	if err := Transactions(&out, &diagnostics, result, false, "never"); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || !strings.Contains(out.String(), "Paper, pens extra") || !strings.Contains(out.String(), "unexplained 2.5000") {
		t.Errorf("human out=%q diagnostics=%q", out.String(), diagnostics.String())
	}
}
