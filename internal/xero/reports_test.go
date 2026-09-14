package xero

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

func TestRecordedReportShapes(t *testing.T) {
	// Recorded September 14, 2026; identities, account labels, tracking names
	// and nonzero amounts are replaced. Native sections and blank cells remain.
	for _, test := range []struct {
		file    string
		columns int
	}{
		{"profit-and-loss", 1}, {"profit-and-loss-by", 10},
		{"balance-sheet", 2}, {"trial-balance", 4}, {"bank-summary", 4},
	} {
		t.Run(test.file, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/reports/" + test.file + ".json")
			if err != nil {
				t.Fatal(err)
			}
			report, err := parseReport(raw)
			if err != nil {
				t.Fatal(err)
			}
			if len(report.Columns) != test.columns || len(report.Rows) == 0 || len(report.Titles) == 0 {
				t.Fatalf("report dimensions = %d columns, %d rows, %d titles", len(report.Columns), len(report.Rows), len(report.Titles))
			}
			accounts, summaries := 0, 0
			for _, row := range report.Rows {
				if len(row.Values) != test.columns || row.Section == nil {
					t.Errorf("row dimensions = %#v", row)
				}
				if row.AccountID != "" {
					accounts++
				}
				if row.Kind == "summary" {
					summaries++
				}
			}
			if accounts == 0 || summaries == 0 {
				t.Errorf("account rows = %d, summaries = %d", accounts, summaries)
			}
		})
	}
}

func TestReportFlatteningPreservesPathsAttributesAndBlanks(t *testing.T) {
	raw := []byte(`{"Reports":[{"ReportName":"Example","ReportTitles":["Title"],"Rows":[
{"RowType":"Header","Cells":[{"Value":"Label"},{"Value":"Current"},{"Value":"Previous"}]},
{"RowType":"Section","Title":"Empty"},
{"RowType":"Section","Title":"Assets","Rows":[
{"RowType":"Section","Title":"Current","Rows":[
{"RowType":"Row","Cells":[{"Value":"Native label"},{"Value":"12345678901234567890.1234","Attributes":[{"Id":"account","Value":"a"}]},{"Value":""}]},
{"RowType":"SummaryRow","Cells":[{"Value":"Total"},{"Value":"12345678901234567890.1234"},{"Value":"0.00"}]}]}]}]}]}`)

	report, err := parseReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	report.ResolveAccounts([]Account{{AccountID: "a", Code: "200", Name: "Current chart name"}})
	want := []ReportRow{
		{AccountCode: "200", AccountID: "a", AccountName: "Current chart name", Kind: "row", Label: "Native label", Section: []string{"Assets", "Current"}, Values: []string{"12345678901234567890.1234", ""}},
		{Kind: "summary", Label: "Total", Section: []string{"Assets", "Current"}, Values: []string{"12345678901234567890.1234", "0.00"}},
	}
	if !reflect.DeepEqual(report.Rows, want) {
		t.Errorf("rows = %#v, want %#v", report.Rows, want)
	}
}

func TestReportRejectsMalformedResponses(t *testing.T) {
	for _, body := range []string{
		`{`, `{"Reports":[]}`, `{"Reports":[{}]}`,
		`{"Reports":[{"Rows":[{"RowType":"Header","Cells":[{},{}]},{"RowType":"Row","Cells":[{}]}]}]}`,
		`{"Reports":[{"Rows":[{"RowType":"Header","Cells":[{}]},{"RowType":"Unexpected","Cells":[{}]}]}]}`,
	} {
		_, err := parseReport([]byte(body))
		if err == nil || apperr.From(err).Code != "api" {
			t.Errorf("parseReport(%q) error = %v", body, err)
		}
	}
}

func TestReportEndpointAndScopeErrors(t *testing.T) {
	for _, scope := range []string{"accounting.reports.read", "accounting.reports.profitandloss.read", "accounting.reports.balancesheet.read"} {
		t.Run(scope, func(t *testing.T) {
			c := testClient(t, map[string]http.HandlerFunc{
				"/api/Reports/ProfitAndLoss": func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Query().Get("fromDate") != "2025-01-01" || r.URL.Query().Get("periods") != "11" || r.URL.Query().Get("timeframe") != "MONTH" {
						t.Errorf("native query = %v", r.URL.Query())
					}
					w.WriteHeader(http.StatusForbidden)
				},
			})
			c.options.Scopes = []string{scope}

			_, err := c.Report(context.Background(), "ProfitAndLoss", url.Values{"fromDate": {"2025-01-01"}, "periods": {"11"}, "timeframe": {"MONTH"}})
			if err == nil || apperr.From(err).Code != "forbidden" {
				t.Fatalf("Report error = %v", err)
			}
			wantRole := scope != "accounting.reports.balancesheet.read"
			if strings.Contains(err.Error(), "Reports role") != wantRole {
				t.Errorf("scope %s error = %v", scope, err)
			}
		})
	}
}

func TestMergeReportColumnsKeepsTotalOrderAndMissingValues(t *testing.T) {
	row := func(section, id, label, value string) ReportRow {
		return ReportRow{Section: []string{section}, AccountID: id, Label: label, Kind: "row", Values: []string{value}}
	}
	reports := []Report{
		{Columns: []string{"date"}, Rows: []ReportRow{row("Assets", "a", "Cash", "1.00"), row("Assets", "c", "Other", "3.00")}},
		{Columns: []string{"date"}, Rows: []ReportRow{row("Assets", "b", "Cash", "2.00")}},
		{Columns: []string{"date"}, Rows: []ReportRow{row("Assets", "b", "Cash", "20.00"), row("Assets", "a", "Cash", "10.00")}},
	}

	merged, err := MergeReportColumns(reports, []string{"North", "South", "Total"})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"", "2.00", "20.00"}, {"1.00", "", "10.00"}, {"3.00", "", ""}}
	for i, row := range merged.Rows {
		if !reflect.DeepEqual(row.Values, want[i]) {
			t.Errorf("row %d = %v, want %v", i, row.Values, want[i])
		}
	}
	if len(merged.Rows) != 3 {
		t.Errorf("rows = %d, want 3", len(merged.Rows))
	}
}

func TestBalanceSheetDateSelection(t *testing.T) {
	report := Report{Columns: []string{"31 Dec 2024", "31 Dec 2025"}, Rows: []ReportRow{{Label: "Cash", Values: []string{"100.00", "200.00"}}}}
	selected, err := report.AtDate("2025-12-31")
	if err != nil || !reflect.DeepEqual(selected.Columns, []string{"31 Dec 2025"}) || !reflect.DeepEqual(selected.Rows[0].Values, []string{"200.00"}) {
		t.Fatalf("AtDate = %#v, %v", selected, err)
	}
	if len(report.Rows[0].Values) != 2 {
		t.Error("AtDate changed the source report")
	}
	if _, err := report.AtDate("2025-11-30"); err == nil {
		t.Error("accepted absent date column")
	}
	report.Columns = []string{"31 Dec 2025", "31 Dec 2025"}
	if _, err := report.AtDate("2025-12-31"); err == nil {
		t.Error("accepted ambiguous date columns")
	}
}

func TestAccountBalanceSignPrecisionAndAbsence(t *testing.T) {
	for _, test := range []struct{ class, want, period string }{
		{"ASSET", "9007199254740992.1234", "cumulative"},
		{"EXPENSE", "9007199254740992.1234", "financial_year_to_date"},
		{"LIABILITY", "-9007199254740992.1234", "cumulative"},
		{"EQUITY", "-9007199254740992.1234", "cumulative"},
		{"REVENUE", "-9007199254740992.1234", "financial_year_to_date"},
	} {
		t.Run(test.class, func(t *testing.T) {
			r := Report{Columns: []string{"Debit", "Credit", "YTD Credit", "YTD Debit"}, Rows: []ReportRow{{AccountID: "a", Values: []string{"99", "88", "1.00", "9007199254740993.1234"}}}}
			balance, err := r.BalanceForAccount(Account{AccountID: "a", Class: test.class}, "2025-12-31", "cash")
			if err != nil || balance.Amount != test.want || balance.Period != test.period || balance.Basis != "cash" || balance.Date != "2025-12-31" {
				t.Fatalf("balance = %#v, %v", balance, err)
			}
			balance, err = r.BalanceForAccount(Account{AccountID: "missing", Class: test.class}, "2025-12-31", "accrual")
			if err != nil || balance.Amount != "0.00" || balance.Note == "" || balance.Period != test.period {
				t.Errorf("absent balance = %#v, %v", balance, err)
			}
		})
	}
	for _, test := range []struct{ left, right, want string }{
		{"", "", "0.00"}, {"0.1", "0.02", "0.08"}, {"-0.001", "0.009", "-0.010"}, {"1.00", "1", "0.00"},
	} {
		got, err := subtractDecimal(test.left, test.right)
		if err != nil || got != test.want {
			t.Errorf("subtractDecimal(%q, %q) = %q, %v; want %q", test.left, test.right, got, err, test.want)
		}
	}
	for _, invalid := range []string{"NaN", "1e3", "1,000.00", "1.2.3"} {
		if _, err := subtractDecimal(invalid, "0"); err == nil {
			t.Errorf("accepted invalid decimal %q", invalid)
		}
	}
}
