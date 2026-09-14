package cmd

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/xero-cli/internal/config"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func TestReportInvalidOptionsPrecedeConfiguration(t *testing.T) {
	for _, test := range []struct {
		args []string
		code int
	}{
		{[]string{"profit-and-loss"}, 2},
		{[]string{"profit-and-loss", "--from", "2025-01-01"}, 2},
		{[]string{"profit-and-loss", "--to", "2025-01-31"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--year", "2025"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--from", "2025-01-01", "--to", "2025-01-31"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--periods", "1"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--timeframe", "month"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--periods", "0", "--timeframe", "month"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--periods", "12", "--timeframe", "month"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--periods", "1", "--timeframe", "day"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--periods", "1", "--timeframe", "month", "--by", "Region"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--basis", "invalid"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--basis", ""}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--csv", "--json"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--tracking", "Region"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--tracking", "Region="}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--tracking", "Region=A", "--tracking", "region=B"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--tracking", "Region=A", "--by", "REGION"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--tracking", "A=a", "--tracking", "B=b", "--tracking", "C=c"}, 2},
		{[]string{"profit-and-loss", "--month", "2025-01", "--by", ""}, 2},
		{[]string{"profit-and-loss", "--month", "2025-13"}, 1},
		{[]string{"profit-and-loss", "--year", "0000"}, 1},
		{[]string{"profit-and-loss", "--from", "2025-02-30", "--to", "2025-03-31"}, 1},
		{[]string{"profit-and-loss", "--from", "2025-03-01", "--to", "2025-02-28"}, 1},
		{[]string{"profit-and-loss", "--from", "2025-01-02", "--to", "2025-01-31", "--periods", "1", "--timeframe", "month"}, 1},
		{[]string{"profit-and-loss", "--month", "2025-03", "--periods", "1", "--timeframe", "quarter"}, 1},
		{[]string{"profit-and-loss", "--month", "2025-01", "--periods", "1", "--timeframe", "year"}, 1},
		{[]string{"balance-sheet", "--date", "2025-02-27", "--periods", "1", "--timeframe", "month"}, 1},
		{[]string{"balance-sheet", "--date", "2025-02-28", "--periods", "1", "--timeframe", "quarter"}, 1},
		{[]string{"trial-balance", "--date", ""}, 1},
		{[]string{"trial-balance", "--tracking", "Region=A"}, 2},
		{[]string{"bank-summary", "--year", "2025"}, 2},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			called := false
			args := append([]string{"--config", "/missing/report-config", "report"}, test.args...)
			code, out, stderr := invoke(args, "", func(string, config.OrgConfig) client {
				called = true
				return &fakeClient{}
			})
			if code != test.code || out != "" || called || strings.Contains(stderr, "report-config") {
				t.Fatalf("code=%d stdout=%q stderr=%q factory=%v", code, out, stderr, called)
			}
		})
	}
}

func TestReportRangeExpansionAndLocalToday(t *testing.T) {
	for _, test := range []struct{ flag, value, from, to string }{
		{"month", "2024-02", "2024-02-01", "2024-02-29"},
		{"month", "2025-02", "2025-02-01", "2025-02-28"},
		{"year", "2024", "2024-01-01", "2024-12-31"},
	} {
		r := reportOptions{name: "ProfitAndLoss", basis: "accrual"}
		command := &cobra.Command{}
		command.Flags().StringVar(&r.month, "month", "", "")
		command.Flags().StringVar(&r.year, "year", "", "")
		if err := command.Flags().Set(test.flag, test.value); err != nil {
			t.Fatal(err)
		}
		query, err := r.validate(command, false, time.Time{})
		if err != nil || query.Get("fromDate") != test.from || query.Get("toDate") != test.to {
			t.Errorf("%s=%s query=%v error=%v", test.flag, test.value, query, err)
		}
	}
	r := reportOptions{name: "TrialBalance", basis: "accrual"}
	command := &cobra.Command{}
	command.Flags().StringVar(&r.date, "date", "", "")
	local := time.Date(2025, 12, 31, 23, 30, 0, 0, time.FixedZone("local", -8*60*60))
	query, err := r.validate(command, false, local)
	if err != nil || query.Get("date") != "2025-12-31" {
		t.Errorf("local date = %v, %v", query, err)
	}
}

func TestReportNativeQueryAndOutputModes(t *testing.T) {
	path := commandConfig(t)
	for _, mode := range []string{"human", "json", "csv"} {
		t.Run(mode, func(t *testing.T) {
			fake := &fakeClient{org: xero.Organisation{Name: "Example Ltd", OrganisationID: "org-id", BaseCurrency: "USD"}, accounts: []xero.Account{{AccountID: "a", Code: "200", Name: "Chart name"}}}
			fake.reportFn = func(name string, query url.Values) (xero.Report, error) {
				want := url.Values{"fromDate": {"2025-01-01"}, "toDate": {"2025-12-31"}, "periods": {"11"}, "timeframe": {"MONTH"}, "paymentsOnly": {"true"}, "standardLayout": {"true"}}
				if name != "ProfitAndLoss" || !reflect.DeepEqual(query, want) {
					t.Errorf("Report(%s, %v), want %v", name, query, want)
				}
				return xero.Report{Name: "Profit and Loss", Columns: []string{"Native caption"}, Rows: []xero.ReportRow{{AccountID: "a", Label: "Native label", Kind: "summary", Section: []string{"Income"}, Values: []string{"1234.5678"}}}}, nil
			}
			args := []string{"--config", path, "--color", "always", "report", "profit-and-loss", "--year", "2025", "--periods", "11", "--timeframe", "month", "--basis", "cash", "--standard-layout"}
			if mode != "human" {
				args = append(args, "--"+mode)
			}
			code, out, stderr := invoke(args, "", func(string, config.OrgConfig) client { return fake })
			if code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
			switch mode {
			case "human":
				if !strings.Contains(out, "1,234.5678") || !strings.Contains(out, "\x1b[1m") || !strings.Contains(out, "cash · USD") || stderr != "" {
					t.Errorf("human=%q stderr=%q", out, stderr)
				}
			case "json":
				var decoded reportOutput
				if err := json.Unmarshal([]byte(out), &decoded); err != nil {
					t.Fatal(err)
				}
				if decoded.Rows[0].AccountCode != "200" || decoded.Rows[0].Values[0] != "1234.5678" || decoded.From != "2025-01-01" || decoded.Org.Name != "Example Ltd" || strings.Count(out, "\n") != 1 || strings.Contains(out, "\x1b") || stderr != "" {
					t.Errorf("json=%s stderr=%q", out, stderr)
				}
			case "csv":
				rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
				if err != nil || !reflect.DeepEqual(rows, [][]string{{"section", "label", "account_code", "Native caption"}, {"Income", "Native label", "200", "1234.5678"}}) || strings.Contains(out+stderr, "\x1b") || !strings.Contains(stderr, "cash · USD") {
					t.Errorf("csv=%q stderr=%q error=%v", out, stderr, err)
				}
			}
		})
	}
}

func TestReportTrackingAndBalanceSheetFanout(t *testing.T) {
	path := commandConfig(t)
	categories := []xero.TrackingCategory{
		{Name: "Region", TrackingCategoryID: "region", Options: []xero.TrackingOption{{Name: "North", TrackingOptionID: "north", Status: "ACTIVE"}, {Name: "South", TrackingOptionID: "south", Status: "ACTIVE"}, {Name: "Old", TrackingOptionID: "old", Status: "ARCHIVED"}}},
		{Name: "Department", TrackingCategoryID: "department", Options: []xero.TrackingOption{{Name: "Sales", TrackingOptionID: "sales", Status: "ACTIVE"}}},
	}
	for _, report := range []string{"profit-and-loss", "balance-sheet"} {
		t.Run(report, func(t *testing.T) {
			calls := 0
			fake := &fakeClient{categories: categories}
			fake.reportFn = func(_ string, query url.Values) (xero.Report, error) {
				calls++
				result := xero.Report{Columns: []string{"31 Dec 2025", "31 Dec 2024"}, Rows: []xero.ReportRow{}}
				if report == "profit-and-loss" {
					if query.Get("trackingCategoryID") != "region" || query.Get("trackingOptionID") != "" || query.Get("trackingCategoryID2") != "department" || query.Get("trackingOptionID2") != "sales" {
						t.Errorf("P&L query = %v", query)
					}
					return result, nil
				}
				if query.Get("trackingOptionID2") != "sales" || query.Get("trackingOptionID1") != []string{"north", "south", ""}[calls-1] {
					t.Errorf("balance sheet call %d query = %v", calls, query)
				}
				if calls != 2 {
					result.Rows = []xero.ReportRow{{AccountID: "a", Label: "Cash", Section: []string{"Assets"}, Kind: "row", Values: []string{"10.00", "999.00"}}}
				}
				return result, nil
			}
			args := []string{"--config", path, "report", report, "--by", "rEgIoN", "--tracking", "DEPARTMENT=sales", "--json"}
			if report == "profit-and-loss" {
				args = append(args, "--month", "2025-12")
			} else {
				args = append(args, "--date", "2025-12-31")
			}
			code, out, stderr := invoke(args, "", func(string, config.OrgConfig) client { return fake })
			if code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
			var result reportOutput
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if report == "balance-sheet" && (calls != 3 || result.By.Note == "" || !reflect.DeepEqual(result.Rows[0].Values, []string{"10.00", "", "10.00"})) {
				t.Errorf("fanout calls=%d output=%s", calls, out)
			}
			if report == "profit-and-loss" && calls != 1 {
				t.Errorf("P&L calls=%d", calls)
			}
		})
	}
	for _, filter := range []string{"Unknown=North", "Region=Unknown"} {
		fake := &fakeClient{categories: categories, reportFn: func(string, url.Values) (xero.Report, error) {
			t.Fatal("report called for unresolved tracking")
			return xero.Report{}, nil
		}}
		code, out, stderr := invoke([]string{"--config", path, "report", "profit-and-loss", "--month", "2025-12", "--tracking", filter}, "", func(string, config.OrgConfig) client { return fake })
		if code != 1 || out != "" || !strings.Contains(stderr, "available:") {
			t.Errorf("unknown tracking code=%d out=%q stderr=%q", code, out, stderr)
		}
	}
	for _, ambiguous := range [][]xero.TrackingCategory{
		{{Name: "Region"}, {Name: "REGION"}},
		{{Name: "Region", Options: []xero.TrackingOption{{Name: "North"}, {Name: "NORTH"}}}},
	} {
		fake := &fakeClient{categories: ambiguous}
		code, out, stderr := invoke([]string{"--config", path, "report", "profit-and-loss", "--month", "2025-12", "--tracking", "Region=North"}, "", func(string, config.OrgConfig) client { return fake })
		if code != 1 || out != "" || !strings.Contains(stderr, "exactly one") {
			t.Errorf("ambiguous tracking code=%d out=%q stderr=%q", code, out, stderr)
		}
	}
}

func TestBankSummaryMetadataAndReportFailures(t *testing.T) {
	path := commandConfig(t)
	fake := &fakeClient{report: xero.Report{Columns: []string{"Opening", "Received", "Spent", "Closing"}, Rows: []xero.ReportRow{}}}
	args := []string{"--config", path, "report", "bank-summary", "--month", "2025-12", "--json"}
	code, out, stderr := invoke(args, "", func(string, config.OrgConfig) client { return fake })
	if code != 0 || stderr != "" || strings.Contains(out, `"basis"`) || strings.Contains(out, `"filters"`) || strings.Contains(out, `"by"`) {
		t.Errorf("bank summary code=%d out=%q stderr=%q", code, out, stderr)
	}
	fake.reportFn = func(string, url.Values) (xero.Report, error) { return xero.Report{}, errors.New("report unavailable") }
	code, out, stderr = invoke(args, "", func(string, config.OrgConfig) client { return fake })
	if code != 1 || out != "" || !strings.Contains(stderr, "report unavailable") {
		t.Errorf("report failure code=%d out=%q stderr=%q", code, out, stderr)
	}
}

func TestTwoReportTrackingFiltersIncludeArchivedOptions(t *testing.T) {
	path := commandConfig(t)
	for _, name := range []string{"profit-and-loss", "balance-sheet"} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeClient{categories: []xero.TrackingCategory{
				{Name: "Region", TrackingCategoryID: "region", Options: []xero.TrackingOption{{Name: "Former", TrackingOptionID: "former", Status: "ARCHIVED"}}},
				{Name: "Team", TrackingCategoryID: "team", Options: []xero.TrackingOption{{Name: "Sales", TrackingOptionID: "sales", Status: "ACTIVE"}}},
			}}
			fake.reportFn = func(_ string, query url.Values) (xero.Report, error) {
				want := url.Values{"trackingOptionID1": {"former"}, "trackingOptionID2": {"sales"}}
				if name == "profit-and-loss" {
					want = url.Values{"trackingCategoryID": {"region"}, "trackingOptionID": {"former"}, "trackingCategoryID2": {"team"}, "trackingOptionID2": {"sales"}}
				}
				for key, values := range want {
					if query.Get(key) != values[0] {
						t.Errorf("%s query[%s] = %q, want %q", name, key, query.Get(key), values[0])
					}
				}
				return xero.Report{Columns: []string{}, Rows: []xero.ReportRow{}}, nil
			}
			args := []string{"--config", path, "report", name, "--tracking", "region=FORMER", "--tracking", "Team=Sales", "--json"}
			if name == "profit-and-loss" {
				args = append(args, "--month", "2025-12")
			}
			code, _, stderr := invoke(args, "", func(string, config.OrgConfig) client { return fake })
			if code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
		})
	}
}

func TestBreakdownFailureLeavesStdoutEmpty(t *testing.T) {
	path := commandConfig(t)
	calls := 0
	fake := &fakeClient{categories: []xero.TrackingCategory{{Name: "Region", Options: []xero.TrackingOption{{Name: "North", TrackingOptionID: "north", Status: "ACTIVE"}}}}}
	fake.reportFn = func(string, url.Values) (xero.Report, error) {
		calls++
		if calls == 2 {
			return xero.Report{}, errors.New("total report unavailable")
		}
		return xero.Report{Columns: []string{"31 Dec 2025"}, Rows: []xero.ReportRow{{Label: "Cash", Values: []string{"1.00"}}}}, nil
	}
	code, out, stderr := invoke([]string{"--config", path, "report", "balance-sheet", "--date", "2025-12-31", "--by", "Region", "--csv"}, "", func(string, config.OrgConfig) client { return fake })
	if code != 1 || calls != 2 || out != "" || !strings.Contains(stderr, "total report unavailable") {
		t.Errorf("breakdown failure code=%d calls=%d stdout=%q stderr=%q", code, calls, out, stderr)
	}
}

func TestAccountShowDateExtendsNativeJSON(t *testing.T) {
	path := commandConfig(t)
	var account xero.Account
	if err := json.Unmarshal([]byte(`{"AccountID":"a","Code":"200","Name":"Sales","Class":"REVENUE","UnknownSourceField":true}`), &account); err != nil {
		t.Fatal(err)
	}
	calls := 0
	fake := &fakeClient{accounts: []xero.Account{account}, reportFn: func(name string, query url.Values) (xero.Report, error) {
		calls++
		if name != "TrialBalance" || query.Get("date") != "2025-12-31" || query.Get("paymentsOnly") != "true" {
			t.Errorf("balance lookup %s %v", name, query)
		}
		return xero.Report{Columns: []string{"YTD Debit", "YTD Credit"}, Rows: []xero.ReportRow{{AccountID: "a", Values: []string{"", "123.45"}}}}, nil
	}}
	args := []string{"--config", path, "account", "show", "200", "--json"}
	code, out, stderr := invoke(args, "", func(string, config.OrgConfig) client { return fake })
	if code != 0 || calls != 0 || strings.Contains(out, `"balance"`) {
		t.Fatalf("plain account code=%d out=%q stderr=%q calls=%d", code, out, stderr, calls)
	}
	code, out, stderr = invoke(append(args, "--date", "2025-12-31", "--basis", "cash"), "", func(string, config.OrgConfig) client { return fake })
	if code != 0 || calls != 1 || !strings.Contains(out, `"UnknownSourceField":true`) || !strings.Contains(out, `"amount":"123.45"`) || !strings.Contains(out, `"period":"financial_year_to_date"`) {
		t.Errorf("dated account code=%d out=%q stderr=%q calls=%d", code, out, stderr, calls)
	}
	code, out, stderr = invoke([]string{"--config", "/missing", "account", "show", "200", "--basis", "cash"}, "", nil)
	if code != 2 || out != "" || !strings.Contains(stderr, "--basis requires --date") {
		t.Errorf("basis without date code=%d out=%q stderr=%q", code, out, stderr)
	}
}
