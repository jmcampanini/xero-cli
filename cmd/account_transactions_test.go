package cmd

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/config"
	"github.com/jmcampanini/xero-cli/internal/xero"
)

func TestAccountTransactionsUsageBeforeStartup(t *testing.T) {
	for _, args := range [][]string{
		{"account", "transactions"},
		{"account", "transactions", "429"},
		{"account", "transactions", "429", "--from", "2025-07-01"},
		{"account", "transactions", "429", "--month", "2025-07", "--from", "2025-07-01", "--to", "2025-07-31"},
		{"account", "transactions", "429", "--month", "2025-07", "--basis", "cash"},
		{"account", "transactions", "429", "--month", "2025-07", "--csv", "--json"},
		{"account", "transactions", "429", "--month", "2025-07", "--tracking", "Region=North", "--tracking", "region=South"},
		{"account", "transactions", "429", "--month", "2025-07", "--tracking", "Region"},
		{"account", "transactions", "429", "--month", "2025-07", "--tracking", "A=1", "--tracking", "B=2", "--tracking", "C=3"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			calls := 0
			code, out, diagnostics := invoke(append([]string{"--config", "/missing/config"}, args...), "", func(string, config.OrgConfig) client { calls++; return &fakeClient{} })
			if code != 2 || out != "" || diagnostics == "" || calls != 0 {
				t.Errorf("usage code=%d out=%q diagnostics=%q clients=%d", code, out, diagnostics, calls)
			}
		})
	}
}

func TestAccountTransactionsCommandTrackingAndOutput(t *testing.T) {
	path := commandConfig(t)
	fake := &fakeClient{
		org:        xero.Organisation{OrganisationID: "org-id", Name: "Example Ltd", BaseCurrency: "USD", FinancialYearEndDay: 31, FinancialYearEndMonth: 12},
		accounts:   []xero.Account{{AccountID: "expense", Code: "429", Name: "Supplies", Type: "EXPENSE", Class: "EXPENSE"}},
		categories: []xero.TrackingCategory{{Name: "Region", Options: []xero.TrackingOption{{Name: "North", Status: "ARCHIVED"}}}},
	}
	var record xero.Record
	if err := json.Unmarshal([]byte(`{"BankTransactionID":"source","Date":"2025-07-02","Status":"AUTHORISED","Type":"SPEND","CurrencyCode":"USD","LineAmountTypes":"NoTax","LineItems":[{"AccountID":"expense","Description":"Paper","LineAmount":"12.50","Tracking":[{"Name":"Region","Option":"North","Extra":42}]}]}`), &record); err != nil {
		t.Fatal(err)
	}
	fake.recordsFn = func(resource string, q xero.ListQuery) (xero.RecordList, error) {
		result := xero.RecordList{Complete: true}
		if resource == "BankTransactions" {
			result.Items = []xero.Record{record}
		}
		return result, nil
	}
	reports := 0
	fake.reportFn = func(string, url.Values) (xero.Report, error) {
		reports++
		return xero.Report{}, apperr.New("api", "unexpected report")
	}
	for _, format := range []string{"--json", "--csv", ""} {
		t.Run(format, func(t *testing.T) {
			args := []string{"--config", path, "account", "transactions", "supplies", "--month", "2025-07", "--tracking", "region=north"}
			if format != "" {
				args = append(args, format)
			}
			code, out, diagnostics := invoke(args, "", func(string, config.OrgConfig) client { return fake })
			if code != 0 || reports != 0 || strings.Contains(out, `"balance":`) {
				t.Fatalf("code=%d out=%q diagnostics=%q reports=%d", code, out, diagnostics, reports)
			}
			switch format {
			case "--json":
				var result xero.AccountTransactions
				if err := json.Unmarshal([]byte(out), &result); err != nil {
					t.Fatal(err)
				}
				if result.NetMovement != "12.50" || result.Filters.Tracking[0].Category != "Region" || result.Filters.Tracking[0].Option != "North" || diagnostics != "" || strings.Count(out, "\n") != 1 {
					t.Errorf("JSON result = %+v, diagnostics %q", result, diagnostics)
				}
			case "--csv":
				if !strings.HasPrefix(out, "date,source_type,source_subtype,source_id,") || !strings.Contains(diagnostics, "net movement 12.50") || strings.Contains(out, "balance") {
					t.Errorf("CSV out=%q diagnostics=%q", out, diagnostics)
				}
			default:
				if !strings.Contains(out, "Paper") || !strings.Contains(out, "net movement 12.50") || !strings.Contains(out, "incomplete coverage") || diagnostics != "" {
					t.Errorf("human out=%q diagnostics=%q", out, diagnostics)
				}
			}
		})
	}
}

func TestAccountTransactionsFailureLeavesStdoutEmpty(t *testing.T) {
	path := commandConfig(t)
	for _, failAt := range []string{"source", "closing"} {
		t.Run(failAt, func(t *testing.T) {
			fake := &fakeClient{org: xero.Organisation{BaseCurrency: "USD", FinancialYearEndMonth: 12, FinancialYearEndDay: 31}, accounts: []xero.Account{{AccountID: "expense", Code: "429", Class: "EXPENSE"}}}
			reports := 0
			fake.reportFn = func(string, url.Values) (xero.Report, error) {
				reports++
				if failAt == "closing" && reports == 2 {
					return xero.Report{}, apperr.New("api", "closing report failed")
				}
				return xero.Report{Columns: []string{"YTD Debit", "YTD Credit"}}, nil
			}
			fake.recordsFn = func(string, xero.ListQuery) (xero.RecordList, error) {
				if failAt == "source" {
					return xero.RecordList{}, apperr.New("api", "source failed")
				}
				return xero.RecordList{Complete: true}, nil
			}
			code, out, diagnostics := invoke([]string{"--config", path, "account", "transactions", "429", "--month", "2025-07", "--json"}, "", func(string, config.OrgConfig) client { return fake })
			if code != 1 || out != "" || !strings.Contains(diagnostics, `"code":"api"`) {
				t.Errorf("failure code=%d out=%q diagnostics=%q", code, out, diagnostics)
			}
		})
	}
}
