package xero

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// TransactionReader supplies the existing read operations used to assemble an account view.
type TransactionReader interface {
	Accounts(context.Context, url.Values) ([]Account, error)
	Records(context.Context, string, ListQuery) (RecordList, error)
	Report(context.Context, string, url.Values) (Report, error)
}

// TrackingFilter describes a resolved category and option using their Xero names.
type TrackingFilter struct {
	Category string `json:"category"`
	Option   string `json:"option"`
}

// TransactionQuery specifies an account, cash period, and optional movement filters.
type TransactionQuery struct {
	Account        Account
	From           string
	IncludeDeleted bool
	Organisation   Organisation
	To             string
	Today          string
	Tracking       []TrackingFilter
}

// TransactionAccount identifies the chart account in the assembled view.
type TransactionAccount struct {
	Class string `json:"class"`
	Code  string `json:"code"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

// TransactionFilters records the requested tracking selection.
type TransactionFilters struct {
	Tracking []TrackingFilter `json:"tracking"`
}

// AccountTransaction is a document or line posting, retaining native tracking objects.
type AccountTransaction struct {
	Balance       string   `json:"balance,omitempty"`
	Contact       string   `json:"contact"`
	Credit        string   `json:"credit"`
	Date          string   `json:"date"`
	Debit         string   `json:"debit"`
	Description   string   `json:"description"`
	Reference     string   `json:"reference"`
	SourceID      string   `json:"source_id"`
	SourceSubtype string   `json:"source_subtype"`
	SourceType    string   `json:"source_type"`
	Status        string   `json:"status"`
	Tracking      []Record `json:"tracking"`
}

// AccountTransactions contains supported cash movements and optional reconstructed balances.
// Complete remains false: source coverage excludes payments and system-generated lines.
type AccountTransactions struct {
	Account        TransactionAccount   `json:"account"`
	Basis          string               `json:"basis"`
	ClosingBalance string               `json:"closing_balance,omitempty"`
	Complete       bool                 `json:"complete"`
	Currency       string               `json:"currency"`
	Filters        TransactionFilters   `json:"filters"`
	From           string               `json:"from"`
	Gap            string               `json:"gap"`
	NetMovement    string               `json:"net_movement"`
	OpeningBalance string               `json:"opening_balance,omitempty"`
	OpeningPeriod  string               `json:"opening_period,omitempty"`
	Org            Identity             `json:"org"`
	Rows           []AccountTransaction `json:"rows"`
	To             string               `json:"to"`
	TrialBalance   string               `json:"trial_balance,omitempty"`
	Unexplained    string               `json:"unexplained,omitempty"`
}

// ReadAccountTransactions retrieves every source before calculating or returning output.
func ReadAccountTransactions(ctx context.Context, reader TransactionReader, q TransactionQuery) (AccountTransactions, error) {
	from, err := ParseCalendarDate(q.From)
	if err != nil {
		return AccountTransactions{}, err
	}
	to, err := ParseCalendarDate(q.To)
	if err != nil {
		return AccountTransactions{}, err
	}
	today, err := ParseCalendarDate(q.Today)
	if err != nil {
		return AccountTransactions{}, err
	}
	if to.Before(from) {
		return AccountTransactions{}, apperr.New("invalid_argument", "--from must not be after --to")
	}
	if q.Account.AccountID == "" || q.Organisation.BaseCurrency == "" {
		return AccountTransactions{}, apperr.New("api", "account ID or organisation base currency is missing")
	}
	if q.Account.Type == "BANK" && q.Account.CurrencyCode == "" {
		return AccountTransactions{}, apperr.New("api", "selected bank account has no currency")
	}
	if err := transactionCurrency(q.Account.CurrencyCode, q.Organisation.BaseCurrency); err != nil {
		return AccountTransactions{}, err
	}
	period, debitPositive, err := accountBalanceConvention(q.Account.Class)
	if err != nil {
		return AccountTransactions{}, err
	}
	opening := AccountBalance{Amount: "0.00", Basis: "cash", Date: q.From, Period: period}
	filtered := len(q.Tracking) > 0
	if !filtered && opening.Period == "financial_year_to_date" {
		start, err := financialYearStart(from, q.Organisation)
		if err != nil {
			return AccountTransactions{}, err
		}
		endStart, err := financialYearStart(to, q.Organisation)
		if err != nil {
			return AccountTransactions{}, err
		}
		if !start.Equal(endStart) {
			return AccountTransactions{}, apperr.New("invalid_argument", "income and expense balances require a range within one financial year; split the range at %s", endStart.Format("2006-01-02"))
		}
	}
	result := AccountTransactions{
		Account: TransactionAccount{Class: q.Account.Class, Code: q.Account.Code, ID: q.Account.AccountID, Name: q.Account.Name, Type: q.Account.Type},
		Basis:   "cash", Currency: q.Organisation.BaseCurrency, Filters: TransactionFilters{Tracking: append([]TrackingFilter{}, q.Tracking...)}, From: q.From,
		Gap:         "invoice/bill payments and refunds, prepayment/overpayment allocations and refunds, and system-generated lines (tax, FX, payroll) are excluded; coverage is incomplete even when unexplained is zero",
		NetMovement: "0.00", Org: Identity{ID: q.Organisation.OrganisationID, Name: q.Organisation.Name}, Rows: []AccountTransaction{}, To: q.To,
	}
	if !filtered {
		atYearStart := false
		if opening.Period == "financial_year_to_date" {
			start, _ := financialYearStart(from, q.Organisation)
			atYearStart = from.Equal(start)
		}
		if !atYearStart {
			previous := from.AddDate(0, 0, -1)
			if previous.Year() < 1 {
				return AccountTransactions{}, apperr.New("invalid_argument", "opening balance requires a date after 0001-01-01")
			}
			opening, err = cashAccountBalance(ctx, reader, q.Account, previous.Format("2006-01-02"))
			if err != nil {
				return AccountTransactions{}, err
			}
		}
		result.OpeningBalance, result.OpeningPeriod = opening.Amount, opening.Period
	}
	rows, err := collectAccountTransactions(ctx, reader, q, from, to)
	if err != nil {
		return AccountTransactions{}, err
	}
	rank := map[string]int{"bank_transaction": 0, "transfer": 1, "manual_journal": 2}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if a.SourceType != b.SourceType {
			return rank[a.SourceType] < rank[b.SourceType]
		}
		return a.SourceID < b.SourceID
	})
	balance := result.OpeningBalance
	for _, row := range rows {
		if !matchesTransactionTracking(row.Tracking, q.Tracking) {
			continue
		}
		if row.Status != "DELETED" && row.Status != "VOIDED" {
			left, right := row.Debit, row.Credit
			if !debitPositive {
				left, right = right, left
			}
			movement, err := subtractDecimal(left, right)
			if err != nil {
				return AccountTransactions{}, err
			}
			result.NetMovement, err = addDecimal(result.NetMovement, movement)
			if err != nil {
				return AccountTransactions{}, err
			}
			if !filtered {
				balance, err = addDecimal(balance, movement)
				if err != nil {
					return AccountTransactions{}, err
				}
			}
		}
		if !filtered {
			row.Balance = balance
		}
		result.Rows = append(result.Rows, row)
	}
	if !filtered {
		result.ClosingBalance = balance
		if !to.After(today) {
			closing, err := cashAccountBalance(ctx, reader, q.Account, q.To)
			if err != nil {
				return AccountTransactions{}, err
			}
			result.TrialBalance = closing.Amount
			result.Unexplained, err = subtractDecimal(closing.Amount, balance)
			if err != nil {
				return AccountTransactions{}, err
			}
		}
	}
	return result, nil
}

func cashAccountBalance(ctx context.Context, reader TransactionReader, account Account, date string) (AccountBalance, error) {
	report, err := reader.Report(ctx, "TrialBalance", url.Values{"date": {date}, "paymentsOnly": {"true"}})
	if err != nil {
		return AccountBalance{}, err
	}
	return report.BalanceForAccount(account, date, "cash")
}

func financialYearStart(date time.Time, org Organisation) (time.Time, error) {
	month, day := time.Month(org.FinancialYearEndMonth), org.FinancialYearEndDay
	if month < time.January || month > time.December || day < 1 || day > time.Date(2000, month+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		return time.Time{}, apperr.New("api", "organisation has an invalid financial-year end")
	}
	end := func(year int) time.Time {
		last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
		return time.Date(year, month, min(day, last), 0, 0, 0, 0, time.UTC)
	}
	year := date.Year()
	if !date.After(end(year)) {
		year--
	}
	return end(year).AddDate(0, 0, 1), nil
}

func addDecimal(left, right string) (string, error) {
	if strings.HasPrefix(right, "-") {
		right = strings.TrimPrefix(right, "-")
	} else if right != "" {
		right = "-" + right
	}
	return subtractDecimal(left, right)
}

func transactionCurrency(currency, base string) error {
	if currency != "" && currency != base {
		return apperr.New("invalid_argument", "account transactions supports only organisation base currency %s; relevant account or transaction uses %s", base, currency)
	}
	return nil
}

func collectAccountTransactions(ctx context.Context, reader TransactionReader, q TransactionQuery, from, to time.Time) ([]AccountTransaction, error) {
	period := fmt.Sprintf("Date>=DateTime(%d,%d,%d)&&Date<=DateTime(%d,%d,%d)", from.Year(), from.Month(), from.Day(), to.Year(), to.Month(), to.Day())
	bank := q.Account.Type == "BANK"
	bankWhere := period
	if bank {
		bankWhere += "&&BankAccount.AccountID==Guid(" + strconv.Quote(q.Account.AccountID) + ")"
	}
	records, err := readTransactionRecords(ctx, reader, "BankTransactions", bankWhere)
	if err != nil {
		return nil, err
	}
	var rows []AccountTransaction
	mirrors := map[string]bool{}
	for _, record := range records {
		if bank && record.Object("BankAccount").Text("AccountID") != q.Account.AccountID {
			continue
		}
		lines := record.Records("LineItems")
		if !bank {
			lines = matchingAccountLines(lines, q.Account)
			if len(lines) == 0 {
				continue
			}
		}
		kind := record.Text("Type")
		debit := false
		switch kind {
		case "SPEND", "SPEND-PREPAYMENT", "SPEND-OVERPAYMENT", "SPEND-TRANSFER":
			debit = !bank
		case "RECEIVE", "RECEIVE-PREPAYMENT", "RECEIVE-OVERPAYMENT", "RECEIVE-TRANSFER":
			debit = bank
		default:
			return nil, apperr.New("api", "unsupported bank transaction type %q", kind)
		}
		row, err := transactionRow(record, "BankTransactionID", "bank_transaction", q)
		if err != nil {
			return nil, err
		}
		if bank && strings.HasSuffix(kind, "-TRANSFER") {
			mirrors[row.SourceID] = true
		}
		if !q.IncludeDeleted && row.Status == "DELETED" {
			continue
		}
		if row.Status != "AUTHORISED" && row.Status != "DELETED" {
			return nil, apperr.New("api", "unsupported bank transaction status %q", row.Status)
		}
		if record.Text("CurrencyCode") == "" {
			return nil, apperr.New("api", "bank transaction %s has no currency", row.SourceID)
		}
		if err := transactionCurrency(record.Text("CurrencyCode"), q.Organisation.BaseCurrency); err != nil {
			return nil, err
		}
		row.SourceSubtype = kind
		row.Contact = record.Object("Contact").Text("Name")
		if bank {
			if len(lines) > 0 {
				row.Description = lines[0].Text("Description")
			}
			if len(lines) > 1 {
				row.Description += fmt.Sprintf(" +%d more", len(lines)-1)
			}
			for _, line := range lines {
				row.Tracking = unionTracking(row.Tracking, line.Records("Tracking"))
			}
			row.Debit, row.Credit, err = transactionSides(record.Text("Total"), debit)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		} else {
			for _, line := range lines {
				item, err := transactionLine(row, record, line, debit)
				if err != nil {
					return nil, err
				}
				rows = append(rows, item)
			}
		}
	}
	if bank {
		return collectTransfers(ctx, reader, q, period, rows, mirrors)
	}
	journals, err := readTransactionRecords(ctx, reader, "ManualJournals", period)
	if err != nil {
		return nil, err
	}
	for _, record := range journals {
		status := record.Text("Status")
		if status == "DRAFT" {
			continue
		}
		lines := matchingAccountLines(record.Records("JournalLines"), q.Account)
		if len(lines) == 0 {
			continue
		}
		if status != "POSTED" && status != "DELETED" && status != "VOIDED" {
			return nil, apperr.New("api", "unsupported manual journal status %q", status)
		}
		if status != "POSTED" && !q.IncludeDeleted {
			continue
		}
		cash := record.Text("ShowOnCashBasisReports")
		if cash == "false" {
			continue
		}
		if cash != "true" && cash != "" {
			return nil, apperr.New("api", "manual journal has invalid ShowOnCashBasisReports")
		}
		row, err := transactionRow(record, "ManualJournalID", "manual_journal", q)
		if err != nil {
			return nil, err
		}
		row.Contact = record.Text("Narration")
		row.SourceSubtype = "MANUALJOURNAL"
		for _, line := range lines {
			item, err := transactionLine(row, record, line, true)
			if err != nil {
				return nil, err
			}
			rows = append(rows, item)
		}
	}
	return rows, nil
}

func readTransactionRecords(ctx context.Context, reader TransactionReader, resource, where string) ([]Record, error) {
	result, err := reader.Records(ctx, resource, ListQuery{Values: url.Values{"where": {where}}})
	if err != nil {
		return nil, err
	}
	if !result.Complete {
		return nil, apperr.New("api", "%s retrieval was incomplete", resource)
	}
	return result.Items, nil
}

func transactionRow(record Record, idKey, source string, q TransactionQuery) (AccountTransaction, error) {
	id, date := record.Text(idKey), record.Text("Date")
	if id == "" {
		return AccountTransaction{}, apperr.New("api", "%s has no source ID", source)
	}
	if _, err := ParseCalendarDate(date); err != nil {
		return AccountTransaction{}, apperr.New("api", "%s %s has an invalid date", source, id)
	}
	if date < q.From || date > q.To {
		return AccountTransaction{}, apperr.New("api", "%s %s returned outside the requested date range", source, id)
	}
	return AccountTransaction{Date: date, Reference: record.Text("Reference"), SourceID: id, SourceType: source, Status: record.Text("Status"), Tracking: []Record{}}, nil
}

func matchingAccountLines(lines []Record, account Account) []Record {
	var matches []Record
	for _, line := range lines {
		id := line.Text("AccountID")
		if id == account.AccountID || id == "" && account.Code != "" && line.Text("AccountCode") == account.Code {
			matches = append(matches, line)
		}
	}
	return matches
}

func transactionLine(row AccountTransaction, record, line Record, debit bool) (AccountTransaction, error) {
	amount := line.Text("LineAmount")
	if !ValidDecimal(amount) {
		return AccountTransaction{}, apperr.New("api", "%s %s has an invalid LineAmount", row.SourceType, row.SourceID)
	}
	switch record.Text("LineAmountTypes") {
	case "Inclusive":
		tax := line.Text("TaxAmount")
		if !ValidDecimal(tax) {
			return AccountTransaction{}, apperr.New("api", "inclusive line has no valid TaxAmount")
		}
		var err error
		amount, err = subtractDecimal(amount, tax)
		if err != nil {
			return AccountTransaction{}, err
		}
	case "Exclusive", "NoTax":
	default:
		return AccountTransaction{}, apperr.New("api", "%s %s has an unsupported LineAmountTypes", row.SourceType, row.SourceID)
	}
	row.Description = line.Text("Description")
	row.Tracking = append([]Record{}, line.Records("Tracking")...)
	var err error
	row.Debit, row.Credit, err = transactionSides(amount, debit)
	return row, err
}

func transactionSides(amount string, debit bool) (string, string, error) {
	if !ValidDecimal(amount) {
		return "", "", apperr.New("api", "transaction has an invalid amount %q", amount)
	}
	if strings.HasPrefix(amount, "-") {
		amount = strings.TrimPrefix(amount, "-")
		debit = !debit
	}
	if debit {
		return amount, "", nil
	}
	return "", amount, nil
}

func unionTracking(target, items []Record) []Record {
	for _, item := range items {
		found := false
		for _, existing := range target {
			if existing.Text("Name") == item.Text("Name") && existing.Text("Option") == item.Text("Option") && existing.Text("TrackingCategoryID") == item.Text("TrackingCategoryID") && existing.Text("TrackingOptionID") == item.Text("TrackingOptionID") {
				// Only remove identical objects; distinct unknown fields remain visible.
				if reflect.DeepEqual(existing, item) {
					found = true
					break
				}
			}
		}
		if !found {
			target = append(target, item)
		}
	}
	return target
}

func matchesTransactionTracking(items []Record, filters []TrackingFilter) bool {
	for _, filter := range filters {
		found := false
		for _, item := range items {
			if strings.EqualFold(item.Text("Name"), filter.Category) && strings.EqualFold(item.Text("Option"), filter.Option) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func collectTransfers(ctx context.Context, reader TransactionReader, q TransactionQuery, period string, rows []AccountTransaction, mirrors map[string]bool) ([]AccountTransaction, error) {
	records, err := readTransactionRecords(ctx, reader, "BankTransfers", period)
	if err != nil {
		return nil, err
	}
	var accounts []Account
	for _, record := range records {
		from, to := record.Object("FromBankAccount"), record.Object("ToBankAccount")
		incoming := to.Text("AccountID") == q.Account.AccountID
		if !incoming && from.Text("AccountID") != q.Account.AccountID {
			continue
		}
		side := "From"
		if incoming {
			side = "To"
		}
		mirrorID := record.Text(side + "BankTransactionID")
		if mirrors[mirrorID] {
			// Xero can return side tracking on the transfer even when its
			// mirrored bank transaction has no line tracking.
			for i := range rows {
				if rows[i].SourceID == mirrorID {
					rows[i].Tracking = unionTracking(rows[i].Tracking, record.Records(side+"Tracking"))
					break
				}
			}
			continue
		}
		status := record.Text("Status")
		if status == "DELETED" && !q.IncludeDeleted {
			continue
		}
		if status != "AUTHORISED" && status != "DELETED" {
			return nil, apperr.New("api", "unsupported bank transfer status %q", status)
		}
		if accounts == nil {
			accounts, err = reader.Accounts(ctx, nil)
			if err != nil {
				return nil, err
			}
		}
		for _, id := range []string{from.Text("AccountID"), to.Text("AccountID")} {
			found := false
			for _, account := range accounts {
				if account.AccountID == id {
					found = true
					if account.CurrencyCode == "" {
						return nil, apperr.New("api", "transfer bank account has no currency")
					}
					if err := transactionCurrency(account.CurrencyCode, q.Organisation.BaseCurrency); err != nil {
						return nil, err
					}
					break
				}
			}
			if !found {
				return nil, apperr.New("api", "transfer bank account was not found in the chart")
			}
		}
		row, err := transactionRow(record, "BankTransferID", "transfer", q)
		if err != nil {
			return nil, err
		}
		row.SourceSubtype = "SPEND-TRANSFER"
		row.Description = "Transfer to " + to.Text("Name")
		if incoming {
			row.SourceSubtype = "RECEIVE-TRANSFER"
			row.Description = "Transfer from " + from.Text("Name")
		}
		row.Tracking = append([]Record{}, record.Records(side+"Tracking")...)
		row.Debit, row.Credit, err = transactionSides(record.Text("Amount"), incoming)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
