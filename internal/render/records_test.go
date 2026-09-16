package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jmcampanini/xero-cli/internal/xero"
)

func TestDocumentDetailTables(t *testing.T) {
	for _, test := range []struct {
		resource, raw string
		want          []string
	}{
		{"BankTransactions", `{"BankTransactionID":"bank","Type":"SPEND","Status":"DELETED","Date":"2025-01-02","Contact":{"Name":"Example Person","ContactID":"contact"},"BankAccount":{"Code":"090","Name":"Checking","AccountID":"account"},"Total":"42.0000","LineItems":[{"AccountCode":"200","Description":"Tracked line","Quantity":1.5,"UnitAmount":"25.1250","LineAmount":"37.6875","TaxAmount":"0.00","TaxType":"NONE","Tracking":[{"Name":"Region","Option":"North"}]},{"AccountCode":"201","Description":"Untracked line","LineAmount":"4.3125","Tracking":[]}],"IsReconciled":true}`, []string{"DELETED", "2025-01-02", "Example Person (contact)", "090 Checking (account)", "25.1250", "37.6875", "0.00", "NONE", "Region=North", "Untracked line"}},
		{"ManualJournals", `{"ManualJournalID":"journal","Status":"VOIDED","Date":"2025-01-02","Narration":"Accrual","ShowOnCashBasisReports":false,"JournalLines":[{"AccountCode":"200","Description":"Debit line","LineAmount":"123.4500","TaxType":"NONE","TaxAmount":"0.00","Tracking":[{"Name":"Region","Option":"North"}]},{"AccountCode":"300","Description":"Credit line","LineAmount":"-123.4500","TaxAmount":"0.00","Tracking":[]}]}`, []string{"VOIDED", "Accrual", "ShowOnCashBasisReports", "no", "DEBIT", "CREDIT", "123.4500", "Region=North"}},
		{"BankTransfers", `{"BankTransferID":"transfer","Date":"2025-01-02","Amount":"25.00","FromBankAccount":{"Name":"Checking","AccountID":"one"},"ToBankAccount":{"Name":"Savings","AccountID":"two"},"FromIsReconciled":true,"ToIsReconciled":false,"Reference":"Sweep"}`, []string{"Checking (one)", "Savings (two)", "25.00", "Sweep", "FromIsReconciled", "ToIsReconciled", "yes", "no"}},
		{"Contacts", `{"ContactID":"contact","Name":"Example","UnknownScalar":"retained scalar","AccountsReceivableTaxType":"NONE","AccountsPayableTaxType":"INPUT","Balances":{"AccountsReceivable":{"Outstanding":"150.25","Overdue":"0.00"},"AccountsPayable":{"Outstanding":"12.00","Overdue":"12.00"}},"Phones":[{"PhoneType":"DEFAULT","PhoneNumber":"555-0100"}],"Addresses":[{"AddressType":"STREET","AddressLine1":"1 Example Road","City":"Example City"}],"SalesTrackingCategories":[{"TrackingCategoryName":"Region","TrackingOptionName":"North"}],"PurchasesTrackingCategories":[{"TrackingCategoryName":"Team","TrackingOptionName":"Sales"}]}`, []string{"retained scalar", "555-0100", "1 Example Road, Example City", "Defaults applied by Xero to new transactions", "Sales tax type", "Purchase tax type", "Region=North", "Team=Sales", "OUTSTANDING", "Receivable", "150.25", "Payable", "12.00"}},
	} {
		t.Run(test.resource, func(t *testing.T) {
			var record xero.Record
			if err := json.Unmarshal([]byte(test.raw), &record); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := Record(&output, test.resource, record, "yes (2)", "never"); err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(output.String(), want) {
					t.Errorf("missing %q in %s", want, output.String())
				}
			}
			if strings.Contains(output.String(), "-123.4500") {
				t.Error("credit amount retained negative sign")
			}
			if test.resource == "ManualJournals" {
				var header, debit, credit string
				for _, line := range strings.Split(output.String(), "\n") {
					switch {
					case strings.Contains(line, "DEBIT"):
						header = line
					case strings.Contains(line, "Debit line"):
						debit = line
					case strings.Contains(line, "Credit line"):
						credit = line
					}
				}
				if strings.Index(debit, "123.4500") != strings.Index(header, "DEBIT") || strings.Index(credit, "123.4500") != strings.Index(header, "CREDIT") {
					t.Errorf("journal amounts are in the wrong columns:\n%s", output.String())
				}
			}
			if strings.Contains(output.String(), "\x1b") {
				t.Error("no-color output has escapes")
			}
		})
	}
}

func TestRecordListsAndNoOutputOnInvalidJournal(t *testing.T) {
	for _, test := range []struct{ resource, raw, want string }{
		{"BankTransactions", `{"Status":"DELETED","Type":"SPEND","Total":"10.00"}`, "SPEND (deleted)"},
		{"ManualJournals", `{"JournalLines":[{"LineAmount":"10.12"},{"LineAmount":"2.88"},{"LineAmount":"-13.00"}]}`, "13.00"},
		{"Contacts", `{"Name":"Example\nName\u001b[31m","EmailAddress":"example@example.invalid","IsCustomer":true,"IsSupplier":false}`, "yes"},
		{"BankTransfers", `{"FromIsReconciled":true,"ToIsReconciled":false}`, "yes/no"},
	} {
		var record xero.Record
		if err := json.Unmarshal([]byte(test.raw), &record); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := Records(&output, xero.Identity{Name: "Example", ID: "org"}, test.resource, []xero.Record{record}, "never"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), test.want) || strings.Contains(output.String(), "\x1b") {
			t.Errorf("output %q", output.String())
		}
	}
	var output bytes.Buffer
	err := Records(&output, xero.Identity{}, "ManualJournals", []xero.Record{{"JournalLines": json.RawMessage(`[{"LineAmount":"bad"}]`)}}, "never")
	if err == nil || output.Len() != 0 {
		t.Errorf("invalid journal = %q, %v", output.String(), err)
	}
}
