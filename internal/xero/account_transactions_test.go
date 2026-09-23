package xero

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

func transactionQuery(account Account) TransactionQuery {
	return TransactionQuery{Account: account, From: "2025-07-01", To: "2025-07-31", Today: "2025-08-01", Organisation: Organisation{OrganisationID: "org-id", Name: "Example Ltd", BaseCurrency: "USD", FinancialYearEndDay: 31, FinancialYearEndMonth: 12}}
}

func fixtureTransactions(t *testing.T) (*Client, *[]string) {
	t.Helper()
	var calls []string
	handlers := map[string]http.HandlerFunc{}
	for resource, file := range map[string]string{"BankTransactions": "bank-transactions", "ManualJournals": "manual-journals", "BankTransfers": "bank-transfers"} {
		body, err := os.ReadFile("testdata/account-transactions/" + file + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var records []json.RawMessage
		if err := json.Unmarshal(body, &records); err != nil {
			t.Fatal(err)
		}
		handlers["/api/"+resource] = func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, resource)
			if !strings.Contains(r.URL.Query().Get("where"), "Date>=DateTime(2025,7,1)") {
				t.Errorf("missing date filter: %s", r.URL.RawQuery)
			}
			envelope := map[string]any{resource: records}
			if resource != "BankTransfers" {
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				if page < 1 || page > 2 {
					t.Errorf("unexpected page %d", page)
					http.Error(w, "bad page", 400)
					return
				}
				midpoint := (len(records) + 1) / 2
				if page == 1 {
					envelope[resource] = records[:midpoint]
				} else {
					envelope[resource] = records[midpoint:]
				}
				envelope["pagination"] = map[string]int{"Page": page, "PageSize": 100, "PageCount": 2, "ItemCount": len(records)}
			}
			if err := json.NewEncoder(w).Encode(envelope); err != nil {
				t.Error(err)
			}
		}
	}
	handlers["/api/Accounts"] = func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{"Accounts": []Account{{AccountID: "bank", CurrencyCode: "USD"}, {AccountID: "other", CurrencyCode: "USD"}}}); err != nil {
			t.Error(err)
		}
	}
	handlers["/api/Reports/TrialBalance"] = func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "TrialBalance")
		if r.URL.Query().Get("paymentsOnly") != "true" {
			t.Error("trial balance must be cash")
		}
		bank, expense := "1000.00", "40.00"
		if r.URL.Query().Get("date") == "2025-07-31" {
			bank, expense = "1170.1250", "200.00"
		}
		_, err := fmt.Fprintf(w, `{"Reports":[{"ReportName":"Trial Balance","Rows":[{"RowType":"Header","Cells":[{"Value":"Account"},{"Value":"YTD Debit"},{"Value":"YTD Credit"}]},{"RowType":"Section","Rows":[{"RowType":"Row","Cells":[{"Value":"Bank","Attributes":[{"Id":"account","Value":"bank"}]},{"Value":%q},{"Value":""}]},{"RowType":"Row","Cells":[{"Value":"Expense","Attributes":[{"Id":"account","Value":"expense"}]},{"Value":%q},{"Value":""}]}]}]}]}`, bank, expense)
		if err != nil {
			t.Error(err)
		}
	}
	return testClient(t, handlers), &calls
}

func TestAccountTransactionsBankPagingAndDeduplication(t *testing.T) {
	c, calls := fixtureTransactions(t)
	q := transactionQuery(Account{AccountID: "bank", Code: "090", Type: "BANK", Class: "ASSET", CurrencyCode: "USD"})
	result, err := ReadAccountTransactions(context.Background(), c, q)
	if err != nil {
		t.Fatal(err)
	}
	var ids, balances []string
	for _, row := range result.Rows {
		ids = append(ids, row.SourceID)
		balances = append(balances, row.Balance)
	}
	if !reflect.DeepEqual(ids, []string{"receive", "negative", "spend", "mirror", "out"}) {
		t.Errorf("source IDs = %v", ids)
	}
	if !reflect.DeepEqual(balances, []string{"1250.00", "1255.1250", "1145.1250", "1195.1250", "1170.1250"}) {
		t.Errorf("running balances = %v", balances)
	}
	if result.Complete || result.OpeningBalance != "1000.00" || result.NetMovement != "170.1250" || result.ClosingBalance != "1170.1250" || result.Unexplained != "0.0000" {
		t.Errorf("totals = %+v", result)
	}
	if strings.Count(strings.Join(*calls, ","), "BankTransactions") != 2 {
		t.Errorf("did not retrieve both pages: %v", *calls)
	}
	if result.Rows[0].Description != "Services +1 more" {
		t.Errorf("description = %q", result.Rows[0].Description)
	}
}

func TestAccountTransactionsExpenseAndDeletedRows(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(strconv.FormatBool(deleted), func(t *testing.T) {
			c, _ := fixtureTransactions(t)
			q := transactionQuery(Account{AccountID: "expense", Code: "429", Type: "EXPENSE", Class: "EXPENSE"})
			q.IncludeDeleted = deleted
			result, err := ReadAccountTransactions(context.Background(), c, q)
			if err != nil {
				t.Fatal(err)
			}
			wantCount := 4
			if deleted {
				wantCount = 6
			}
			if len(result.Rows) != wantCount || result.OpeningBalance != "40.00" || result.ClosingBalance != "151.8750" || result.Unexplained != "48.1250" {
				t.Errorf("expense result = %+v", result)
			}
			var posted []string
			for _, row := range result.Rows {
				if row.SourceID == "posted" {
					posted = append(posted, row.Description)
				}
				if row.SourceID == "spend" && row.Debit != "100.00" {
					t.Errorf("tax-inclusive debit = %q", row.Debit)
				}
			}
			if !reflect.DeepEqual(posted, []string{"First", "Second"}) {
				t.Errorf("source line order = %v", posted)
			}
		})
	}
}

func TestAccountTransactionsTrackingMovementsOnly(t *testing.T) {
	for _, bank := range []bool{false, true} {
		t.Run(strconv.FormatBool(bank), func(t *testing.T) {
			c, calls := fixtureTransactions(t)
			account := Account{AccountID: "expense", Code: "429", Type: "EXPENSE", Class: "EXPENSE"}
			if bank {
				account = Account{AccountID: "bank", Type: "BANK", Class: "ASSET", CurrencyCode: "USD"}
			}
			q := transactionQuery(account)
			q.Tracking = []TrackingFilter{{Category: "region", Option: "north"}}
			if bank {
				q.Tracking = append(q.Tracking, TrackingFilter{Category: "Department", Option: "Office"})
			}
			result, err := ReadAccountTransactions(context.Background(), c, q)
			if err != nil {
				t.Fatal(err)
			}
			want := "100.00"
			if bank {
				want = "250.00"
			}
			if len(result.Rows) != 1 || result.NetMovement != want {
				t.Errorf("filtered movements = %+v", result)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &object); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"opening_balance", "opening_period", "closing_balance", "trial_balance", "unexplained"} {
				if _, present := object[field]; present {
					t.Errorf("unexpected %s in %s", field, encoded)
				}
			}
			if strings.Contains(string(encoded), `"balance":`) {
				t.Errorf("filtered row has a balance: %s", encoded)
			}
			if strings.Contains(strings.Join(*calls, ","), "TrialBalance") {
				t.Errorf("filtered reports called: %v", *calls)
			}
			if !bank && (!strings.Contains(string(encoded), `"UnknownSourceField":"retained"`) || !strings.Contains(string(encoded), `"TrackingOptionID":"north"`)) {
				t.Errorf("native tracking lost: %s", encoded)
			}
		})
	}
}

type transactionStub struct {
	accounts     []Account
	items        map[string][]Record
	report       Report
	reportCalls  []string
	failResource string
	incomplete   bool
}

func (s *transactionStub) Accounts(context.Context, url.Values) ([]Account, error) {
	return s.accounts, nil
}
func (s *transactionStub) Records(_ context.Context, resource string, _ ListQuery) (RecordList, error) {
	if resource == s.failResource {
		return RecordList{}, apperr.New("rate_limited", "retry budget exhausted")
	}
	return RecordList{Complete: !s.incomplete, Items: s.items[resource]}, nil
}
func (s *transactionStub) Report(_ context.Context, _ string, q url.Values) (Report, error) {
	s.reportCalls = append(s.reportCalls, q.Get("date"))
	return s.report, nil
}
func transactionRecord(t *testing.T, value string) Record {
	t.Helper()
	var record Record
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func TestAccountTransactionsOpeningAndClassSigns(t *testing.T) {
	for _, class := range []string{"ASSET", "EXPENSE", "LIABILITY", "EQUITY", "REVENUE"} {
		t.Run(class, func(t *testing.T) {
			s := &transactionStub{report: Report{Columns: []string{"YTD Debit", "YTD Credit"}, Rows: []ReportRow{{AccountID: "a", Values: []string{"12.50", "2.25"}}}}}
			q := transactionQuery(Account{AccountID: "a", Class: class})
			result, err := ReadAccountTransactions(context.Background(), s, q)
			if err != nil {
				t.Fatal(err)
			}
			want := "-10.25"
			if class == "ASSET" || class == "EXPENSE" {
				want = "10.25"
			}
			if result.OpeningBalance != want || result.ClosingBalance != want || result.NetMovement != "0.00" {
				t.Errorf("class %s = %+v", class, result)
			}
		})
	}
	for _, firstDay := range []bool{false, true} {
		s := &transactionStub{report: Report{Columns: []string{"YTD Debit", "YTD Credit"}}}
		q := transactionQuery(Account{AccountID: "a", Class: "EXPENSE"})
		if firstDay {
			q.Organisation.FinancialYearEndMonth = 6
			q.Organisation.FinancialYearEndDay = 30
		}
		result, err := ReadAccountTransactions(context.Background(), s, q)
		if err != nil {
			t.Fatal(err)
		}
		if result.OpeningBalance != "0.00" {
			t.Errorf("missing/start opening = %q", result.OpeningBalance)
		}
		if firstDay && !reflect.DeepEqual(s.reportCalls, []string{"2025-07-31"}) {
			t.Errorf("year-start reports = %v", s.reportCalls)
		}
	}
}

func TestAccountTransactionsFiscalAndFuturePeriods(t *testing.T) {
	for _, tc := range []struct {
		name, class         string
		filtered, wantError bool
	}{
		{"expense cross year", "EXPENSE", false, true}, {"revenue cross year", "REVENUE", false, true}, {"asset cross year", "ASSET", false, false}, {"filtered cross year", "EXPENSE", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &transactionStub{report: Report{Columns: []string{"YTD Debit", "YTD Credit"}}}
			q := transactionQuery(Account{AccountID: "a", Class: tc.class})
			q.From = "2025-12-01"
			q.To = "2026-01-31"
			q.Today = "2026-02-01"
			if tc.filtered {
				q.Tracking = []TrackingFilter{{Category: "Region", Option: "North"}}
			}
			_, err := ReadAccountTransactions(context.Background(), s, q)
			if (err != nil) != tc.wantError {
				t.Errorf("error = %v, wantError %v", err, tc.wantError)
			}
		})
	}
	s := &transactionStub{report: Report{Columns: []string{"YTD Debit", "YTD Credit"}}}
	q := transactionQuery(Account{AccountID: "a", Class: "ASSET"})
	q.Today = "2025-07-01"
	result, err := ReadAccountTransactions(context.Background(), s, q)
	if err != nil {
		t.Fatal(err)
	}
	if result.TrialBalance != "" || result.Unexplained != "" || len(s.reportCalls) != 1 {
		t.Errorf("future comparison = %+v, calls %v", result, s.reportCalls)
	}
}

func TestAccountTransactionsFailuresDoNotReturnPartialView(t *testing.T) {
	good := `{"BankTransactionID":"b","Date":"2025-07-03","Status":"AUTHORISED","Type":"SPEND","CurrencyCode":"USD","LineAmountTypes":"NoTax","LineItems":[{"AccountID":"a","LineAmount":"12.00"}]}`
	for _, tc := range []struct {
		name, record, fail, currency string
		incomplete                   bool
	}{
		{name: "source failure", record: good, fail: "ManualJournals"},
		{name: "incomplete", record: good, incomplete: true},
		{name: "foreign selected", record: good, currency: "EUR"},
		{name: "foreign relevant", record: strings.Replace(good, "USD", "EUR", 1)},
		{name: "missing currency", record: strings.Replace(good, `"CurrencyCode":"USD",`, "", 1)},
		{name: "invalid money", record: strings.Replace(good, "12.00", "bad", 1)},
		{name: "invalid date", record: strings.Replace(good, "2025-07-03", "bad", 1)},
		{name: "inclusive missing tax", record: strings.Replace(good, "NoTax", "Inclusive", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &transactionStub{items: map[string][]Record{"BankTransactions": {transactionRecord(t, tc.record)}}, report: Report{Columns: []string{"YTD Debit", "YTD Credit"}}, failResource: tc.fail, incomplete: tc.incomplete}
			q := transactionQuery(Account{AccountID: "a", Class: "EXPENSE", CurrencyCode: tc.currency})
			result, err := ReadAccountTransactions(context.Background(), s, q)
			if err == nil || !reflect.DeepEqual(result, AccountTransactions{}) {
				t.Errorf("failed read = %+v, %v", result, err)
			}
		})
	}
	s := &transactionStub{items: map[string][]Record{"BankTransactions": {transactionRecord(t, strings.ReplaceAll(strings.Replace(good, "USD", "EUR", 1), `"AccountID":"a"`, `"AccountID":"other"`))}}, report: Report{Columns: []string{"YTD Debit", "YTD Credit"}}}
	if _, err := ReadAccountTransactions(context.Background(), s, transactionQuery(Account{AccountID: "a", Class: "EXPENSE"})); err != nil {
		t.Errorf("unrelated foreign transaction: %v", err)
	}
}

func TestAccountTransactionsClassSignedMovements(t *testing.T) {
	spend := transactionRecord(t, `{"BankTransactionID":"s","Date":"2025-07-01","Status":"AUTHORISED","Type":"SPEND","CurrencyCode":"USD","LineAmountTypes":"NoTax","LineItems":[{"AccountID":"a","LineAmount":"10.00"}]}`)
	receive := transactionRecord(t, `{"BankTransactionID":"r","Date":"2025-07-02","Status":"AUTHORISED","Type":"RECEIVE","CurrencyCode":"USD","LineAmountTypes":"NoTax","LineItems":[{"AccountID":"a","LineAmount":"4.00"}]}`)
	for _, class := range []string{"ASSET", "EXPENSE", "LIABILITY", "EQUITY", "REVENUE"} {
		t.Run(class, func(t *testing.T) {
			s := &transactionStub{items: map[string][]Record{"BankTransactions": {spend, receive}}, report: Report{Columns: []string{"YTD Debit", "YTD Credit"}}}
			result, err := ReadAccountTransactions(context.Background(), s, transactionQuery(Account{AccountID: "a", Class: class}))
			if err != nil {
				t.Fatal(err)
			}
			want := "-6.00"
			if class == "ASSET" || class == "EXPENSE" {
				want = "6.00"
			}
			if result.NetMovement != want || result.ClosingBalance != want || result.Rows[0].Debit != "10.00" || result.Rows[1].Credit != "4.00" {
				t.Errorf("class %s movements = %+v", class, result)
			}
		})
	}
}

func TestAccountTransactionsTransferStatusAndCurrency(t *testing.T) {
	transfer := `{"BankTransferID":"transfer","Date":"2025-07-02","Status":"AUTHORISED","Amount":"25.00","FromBankAccount":{"AccountID":"bank"},"ToBankAccount":{"AccountID":"other"},"FromBankTransactionID":"mirror","FromTracking":[{"Name":"Region","Option":"North"}]}`
	for _, tc := range []struct {
		name, status, currency       string
		include, mirrored, wantError bool
		count                        int
		closing                      string
	}{
		{name: "active", status: "AUTHORISED", currency: "USD", count: 1, closing: "-25.00"},
		{name: "deleted hidden", status: "DELETED", currency: "USD", closing: "0.00"},
		{name: "deleted shown", status: "DELETED", currency: "USD", include: true, count: 1, closing: "0.00"},
		{name: "deleted mirror hidden", status: "DELETED", currency: "USD", mirrored: true, closing: "0.00"},
		{name: "deleted mirror shown", status: "DELETED", currency: "USD", include: true, mirrored: true, count: 1, closing: "0.00"},
		{name: "foreign opposite bank", status: "AUTHORISED", currency: "EUR", wantError: true},
		{name: "missing opposite currency", status: "AUTHORISED", wantError: true},
		{name: "unknown status", status: "OTHER", currency: "USD", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &transactionStub{accounts: []Account{{AccountID: "bank", CurrencyCode: "USD"}, {AccountID: "other", CurrencyCode: tc.currency}}, items: map[string][]Record{"BankTransfers": {transactionRecord(t, strings.Replace(transfer, "AUTHORISED", tc.status, 1))}}, report: Report{Columns: []string{"YTD Debit", "YTD Credit"}}}
			if tc.mirrored {
				s.items["BankTransactions"] = []Record{transactionRecord(t, `{"BankTransactionID":"mirror","Date":"2025-07-02","Status":"DELETED","Type":"SPEND-TRANSFER","CurrencyCode":"USD","BankAccount":{"AccountID":"bank"},"Total":"25.00","LineItems":[]}`)}
			}
			q := transactionQuery(Account{AccountID: "bank", Type: "BANK", Class: "ASSET", CurrencyCode: "USD"})
			q.IncludeDeleted = tc.include
			result, err := ReadAccountTransactions(context.Background(), s, q)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v wantError=%v", err, tc.wantError)
			}
			if !tc.wantError && (len(result.Rows) != tc.count || result.ClosingBalance != tc.closing) {
				t.Errorf("transfer result = %+v", result)
			}
		})
	}
}

func TestAccountTransactionsRetainsTrackingOnMirroredTransfers(t *testing.T) {
	s := &transactionStub{items: map[string][]Record{
		"BankTransactions": {transactionRecord(t, `{"BankTransactionID":"mirror","Date":"2025-07-02","Status":"AUTHORISED","Type":"RECEIVE-TRANSFER","CurrencyCode":"USD","BankAccount":{"AccountID":"bank"},"Total":"25.00","LineItems":[]}`)},
		"BankTransfers":    {transactionRecord(t, `{"BankTransferID":"transfer","Date":"2025-07-02","Status":"AUTHORISED","Amount":"25.00","FromBankAccount":{"AccountID":"other"},"ToBankAccount":{"AccountID":"bank"},"ToBankTransactionID":"mirror","ToTracking":[{"Name":"Region","Option":"North","Extra":"kept"}],"FromTracking":[{"Name":"Region","Option":"South"}]}`)},
	}}
	q := transactionQuery(Account{AccountID: "bank", Type: "BANK", Class: "ASSET", CurrencyCode: "USD"})
	q.Tracking = []TrackingFilter{{Category: "Region", Option: "North"}}
	result, err := ReadAccountTransactions(context.Background(), s, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 || result.Rows[0].SourceID != "mirror" || result.NetMovement != "25.00" || result.Rows[0].Tracking[0].Text("Extra") != "kept" {
		t.Errorf("mirrored tracking result = %+v", result)
	}
	q.Tracking[0].Option = "South"
	result, err = ReadAccountTransactions(context.Background(), s, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("opposite side's tracking matched: %+v", result.Rows)
	}
}
