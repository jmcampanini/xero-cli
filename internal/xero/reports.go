package xero

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// Report retains Xero's column order and flattens its section hierarchy.
type Report struct {
	Columns []string
	Name    string
	Rows    []ReportRow
	Titles  []string
}

// AtDate selects the requested balance-sheet date for a tracking breakdown.
// Xero returns a previous-year column even when periods is omitted or zero.
func (r Report) AtDate(date string) (Report, error) {
	column := -1
	for i, caption := range r.Columns {
		for _, layout := range []string{"2 Jan 2006", "02 Jan 2006", "2006-01-02"} {
			parsed, err := time.Parse(layout, caption)
			if err == nil && parsed.Format("2006-01-02") == date {
				if column >= 0 && column != i {
					return Report{}, apperr.New("api", "balance sheet repeats date column %q", date)
				}
				column = i
				break
			}
		}
	}
	if column < 0 {
		return Report{}, apperr.New("api", "balance sheet has no column for %s", date)
	}
	selected := Report{Name: r.Name, Titles: r.Titles, Columns: []string{r.Columns[column]}, Rows: []ReportRow{}}
	for _, row := range r.Rows {
		if len(row.Values) != len(r.Columns) {
			return Report{}, apperr.New("api", "balance sheet has inconsistent row values")
		}
		row.Values = []string{row.Values[column]}
		selected.Rows = append(selected.Rows, row)
	}
	return selected, nil
}

// ReportRow is a report line with decimal strings exactly as returned by Xero.
type ReportRow struct {
	AccountCode string   `json:"account_code"`
	AccountID   string   `json:"account_id"`
	AccountName string   `json:"-"`
	Kind        string   `json:"kind"`
	Label       string   `json:"label"`
	Section     []string `json:"section"`
	Values      []string `json:"values"`
}

type reportCell struct {
	Attributes []struct{ ID, Value string }
	Value      string
}

type reportNode struct {
	Cells   []reportCell
	Rows    []reportNode
	RowType string
	Title   string
}

// Report retrieves one native report without changing its query semantics.
func (c *Client) Report(ctx context.Context, name string, query url.Values) (Report, error) {
	switch name {
	case "ProfitAndLoss", "BalanceSheet", "TrialBalance", "BankSummary":
	default:
		return Report{}, apperr.New("invalid_argument", "unknown report %q", name)
	}
	status, _, body, err := c.Do(ctx, http.MethodGet, "Reports/"+name, query, nil)
	if err != nil {
		if status == http.StatusForbidden && c.hasReportScope(name) {
			return Report{}, apperr.New("forbidden", "%s; the token carries the report scope; check that the Xero user has the Reports role", err)
		}
		return Report{}, err
	}
	return parseReport(body)
}

func (c *Client) hasReportScope(name string) bool {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	scopes := c.options.Scopes
	if c.token != nil {
		if granted, ok := c.token.Extra("scope").(string); ok {
			scopes = strings.Fields(granted)
		}
	}
	for _, scope := range scopes {
		if scope == "accounting.reports.read" || scope == "accounting.reports."+strings.ToLower(name)+".read" {
			return true
		}
	}
	return false
}

func parseReport(body []byte) (Report, error) {
	var envelope struct {
		Reports []struct {
			ReportName   string
			ReportTitles []string
			Rows         []reportNode
		}
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Report{}, apperr.New("api", "decode report: %v", err)
	}
	if len(envelope.Reports) != 1 {
		return Report{}, apperr.New("api", "expected one report, received %d", len(envelope.Reports))
	}
	source := envelope.Reports[0]
	if len(source.Rows) == 0 || source.Rows[0].RowType != "Header" || len(source.Rows[0].Cells) == 0 {
		return Report{}, apperr.New("api", "report has no column header")
	}
	report := Report{Name: source.ReportName, Titles: source.ReportTitles, Columns: []string{}, Rows: []ReportRow{}}
	for _, cell := range source.Rows[0].Cells[1:] {
		report.Columns = append(report.Columns, cell.Value)
	}
	if err := report.flatten(source.Rows[1:], nil); err != nil {
		return Report{}, err
	}
	return report, nil
}

func (r *Report) flatten(nodes []reportNode, section []string) error {
	for _, node := range nodes {
		if node.RowType == "Section" {
			path := append([]string{}, section...)
			if node.Title != "" {
				path = append(path, node.Title)
			}
			if err := r.flatten(node.Rows, path); err != nil {
				return err
			}
			continue
		}
		kind := "row"
		switch node.RowType {
		case "SummaryRow":
			kind = "summary"
		case "Row", "Header":
		default:
			return apperr.New("api", "unknown report row type %q", node.RowType)
		}
		if len(node.Cells) != len(r.Columns)+1 {
			return apperr.New("api", "report row has %d cells, expected %d", len(node.Cells), len(r.Columns)+1)
		}
		row := ReportRow{Kind: kind, Label: node.Cells[0].Value, Section: append([]string{}, section...), Values: []string{}}
		for _, cell := range node.Cells {
			for _, attribute := range cell.Attributes {
				if attribute.ID == "account" {
					if row.AccountID != "" && row.AccountID != attribute.Value {
						return apperr.New("api", "report row contains conflicting account IDs")
					}
					row.AccountID = attribute.Value
				}
			}
		}
		for _, cell := range node.Cells[1:] {
			row.Values = append(row.Values, cell.Value)
		}
		r.Rows = append(r.Rows, row)
	}
	return nil
}

// ResolveAccounts adds chart metadata without replacing native report labels.
func (r *Report) ResolveAccounts(accounts []Account) {
	byID := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		byID[account.AccountID] = account
	}
	for i := range r.Rows {
		account := byID[r.Rows[i].AccountID]
		r.Rows[i].AccountCode = account.Code
		r.Rows[i].AccountName = account.Name
	}
}

// MergeReportColumns combines single-column reports, preserving the total's order.
// Rows only present in filtered reports are appended in first-seen order.
func MergeReportColumns(reports []Report, columns []string) (Report, error) {
	if len(reports) == 0 || len(reports) != len(columns) {
		return Report{}, apperr.New("api", "report breakdown has inconsistent columns")
	}
	total := reports[len(reports)-1]
	merged := Report{Name: total.Name, Titles: total.Titles, Columns: columns, Rows: []ReportRow{}}
	indices := make(map[string]int)
	order := []int{len(reports) - 1}
	for i := range len(reports) - 1 {
		order = append(order, i)
	}
	for _, column := range order {
		report := reports[column]
		if len(report.Columns) != 1 {
			return Report{}, apperr.New("api", "expected one value column for tracking breakdown, received %d", len(report.Columns))
		}
		seen := make(map[string]bool)
		for _, row := range report.Rows {
			identity := row.AccountID
			if identity == "" {
				identity = row.Label
			}
			keyBytes, _ := json.Marshal(struct {
				Identity string
				Section  []string
			}{Identity: identity, Section: row.Section})
			key := string(keyBytes)
			if seen[key] {
				return Report{}, apperr.New("api", "ambiguous duplicate report row %q in tracking breakdown", row.Label)
			}
			seen[key] = true
			index, exists := indices[key]
			if !exists {
				index = len(merged.Rows)
				indices[key] = index
				copyRow := row
				copyRow.Values = make([]string, len(columns))
				merged.Rows = append(merged.Rows, copyRow)
			}
			if len(row.Values) != 1 {
				return Report{}, apperr.New("api", "tracking breakdown row has inconsistent values")
			}
			merged.Rows[index].Values[column] = row.Values[0]
		}
	}
	return merged, nil
}
