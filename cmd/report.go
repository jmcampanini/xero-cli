package cmd

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func newReport(o *options) *cobra.Command {
	command := &cobra.Command{Use: "report", Short: "Read financial reports", Args: cobra.NoArgs,
		Long: `Read profit and loss, balance sheet, trial balance and bank summary reports.
Each command preserves Xero's report columns and decimal values. Human
tables retain sections; --json produces flat rows and --csv exports a table.
Report requests require report access and the Xero user's Reports role.

` + groupHelp,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
	command.AddCommand(newReportProfitAndLoss(o), newReportBalanceSheet(o), newReportTrialBalance(o), newReportBankSummary(o))
	return command
}

type reportOptions struct {
	basis          string
	by             string
	csv            bool
	date           string
	from           string
	month          string
	name           string
	periods        int
	standardLayout bool
	timeframe      string
	to             string
	tracking       []string
	year           string
}

type reportTracking struct {
	Category string `json:"category"`
	Option   string `json:"option"`
}

type reportFilters struct {
	Tracking []reportTracking `json:"tracking"`
}

type reportBy struct {
	Category string `json:"category"`
	Note     string `json:"note,omitempty"`
}

type reportOutput struct {
	Basis    string           `json:"basis,omitempty"`
	By       *reportBy        `json:"by"`
	Columns  []string         `json:"columns"`
	Currency string           `json:"currency"`
	Date     string           `json:"date,omitempty"`
	Filters  *reportFilters   `json:"filters"`
	From     string           `json:"from,omitempty"`
	Org      xero.Identity    `json:"org"`
	Report   string           `json:"report"`
	Rows     []xero.ReportRow `json:"rows"`
	To       string           `json:"to,omitempty"`
}

func (r *reportOptions) validate(command *cobra.Command, jsonOutput bool, now time.Time) (url.Values, error) {
	if r.csv && jsonOutput {
		return nil, usage("--csv and --json cannot be combined")
	}
	if r.name != "BankSummary" && r.basis != "cash" && r.basis != "accrual" {
		return nil, usage("--basis must be cash or accrual")
	}
	query := make(url.Values)
	if r.basis == "cash" {
		query.Set("paymentsOnly", "true")
	}
	if r.standardLayout {
		query.Set("standardLayout", "true")
	}
	if r.name == "BalanceSheet" || r.name == "TrialBalance" {
		if !command.Flags().Changed("date") {
			r.date = now.Format("2006-01-02")
		}
		if _, err := xero.ParseCalendarDate(r.date); err != nil {
			return nil, err
		}
		query.Set("date", r.date)
	} else {
		if err := r.expandRange(command); err != nil {
			return nil, err
		}
		query.Set("fromDate", r.from)
		query.Set("toDate", r.to)
	}
	if err := r.validateComparisons(command); err != nil {
		return nil, err
	}
	if command.Flags().Changed("periods") {
		query.Set("periods", strconv.Itoa(r.periods))
		query.Set("timeframe", strings.ToUpper(r.timeframe))
	}
	if err := r.validateTracking(command); err != nil {
		return nil, err
	}
	return query, nil
}

func (r *reportOptions) expandRange(command *cobra.Command) error {
	flags := command.Flags()
	if flags.Changed("from") != flags.Changed("to") {
		return usage("--from and --to must be given together")
	}
	forms := 0
	for _, name := range []string{"from", "month", "year"} {
		if flags.Changed(name) {
			forms++
		}
	}
	if forms != 1 {
		return usage("choose exactly one period: --from/--to, --month, or --year where supported")
	}
	if flags.Changed("month") {
		start, err := time.Parse("2006-01", r.month)
		if err != nil || start.Format("2006-01") != r.month || start.Year() < 1 {
			return apperr.New("invalid_argument", "invalid month %q; expected YYYY-MM", r.month)
		}
		r.from, r.to = start.Format("2006-01-02"), start.AddDate(0, 1, -1).Format("2006-01-02")
	}
	if flags.Changed("year") {
		start, err := time.Parse("2006", r.year)
		if err != nil || start.Year() < 1 {
			return apperr.New("invalid_argument", "invalid year %q; expected YYYY", r.year)
		}
		r.from, r.to = r.year+"-01-01", r.year+"-12-31"
	}
	start, err := xero.ParseCalendarDate(r.from)
	if err != nil {
		return err
	}
	end, err := xero.ParseCalendarDate(r.to)
	if err != nil {
		return err
	}
	if end.Before(start) {
		return apperr.New("invalid_argument", "--from must not be after --to")
	}
	return nil
}

func (r *reportOptions) validateComparisons(command *cobra.Command) error {
	flags := command.Flags()
	if flags.Changed("periods") != flags.Changed("timeframe") {
		return usage("--periods and --timeframe must be given together")
	}
	if !flags.Changed("periods") {
		return nil
	}
	if r.periods < 1 || r.periods > 11 {
		return usage("--periods must be between 1 and 11")
	}
	if r.timeframe != "month" && r.timeframe != "quarter" && r.timeframe != "year" {
		return usage("--timeframe must be month, quarter or year")
	}
	if flags.Changed("by") {
		return usage("--by and --periods cannot be combined")
	}
	if r.name == "BalanceSheet" {
		date, _ := xero.ParseCalendarDate(r.date)
		if !periodStart(date.AddDate(0, 0, 1), r.timeframe) {
			return apperr.New("invalid_argument", "--date must be a %s end for a %s comparison", r.timeframe, r.timeframe)
		}
		return nil
	}
	start, _ := xero.ParseCalendarDate(r.from)
	end, _ := xero.ParseCalendarDate(r.to)
	if !periodStart(start, r.timeframe) || !periodStart(end.AddDate(0, 0, 1), r.timeframe) {
		adjective := map[string]string{"month": "monthly", "quarter": "quarterly", "year": "yearly"}[r.timeframe]
		return apperr.New("invalid_argument", "--from/--to must cover whole %ss for a %s comparison", r.timeframe, adjective)
	}
	return nil
}

func periodStart(date time.Time, timeframe string) bool {
	if date.Day() != 1 {
		return false
	}
	switch timeframe {
	case "quarter":
		return (int(date.Month())-1)%3 == 0
	case "year":
		return date.Month() == time.January
	default:
		return true
	}
}

func (r *reportOptions) validateTracking(command *cobra.Command) error {
	if r.by != "" && len(r.tracking) > 1 {
		return usage("--by allows at most one --tracking filter on the other category")
	}
	if len(r.tracking) > 2 {
		return usage("--tracking may be given at most twice")
	}
	if command.Flags().Changed("by") && strings.TrimSpace(r.by) == "" {
		return usage("--by requires a category name")
	}
	var categories []string
	for _, filter := range r.tracking {
		category, option, found := strings.Cut(filter, "=")
		if !found || strings.TrimSpace(category) == "" || strings.TrimSpace(option) == "" {
			return usage("--tracking must be CATEGORY=OPTION")
		}
		for _, previous := range categories {
			if strings.EqualFold(previous, category) {
				return usage("--tracking cannot repeat category %q", category)
			}
		}
		if r.by != "" && strings.EqualFold(r.by, category) {
			return usage("--by and --tracking cannot use the same category")
		}
		categories = append(categories, category)
	}
	return nil
}

func (o *options) runReport(r *reportOptions) func(*cobra.Command, []string) error {
	return o.run(func(command *cobra.Command, _ []string) error {
		query, err := r.validate(command, o.json, time.Now())
		if err != nil {
			return err
		}
		api, err := o.selectedClient(command)
		if err != nil {
			return err
		}
		ctx := command.Context()
		org, err := api.Organisation(ctx)
		if err != nil {
			return err
		}
		output := reportOutput{Basis: r.basis, Currency: org.BaseCurrency, Date: r.date, From: r.from,
			Org: xero.Identity{Name: org.Name, ID: org.OrganisationID}, Report: r.name, To: r.to,
			Filters: &reportFilters{Tracking: []reportTracking{}},
		}
		category, err := r.resolveTracking(ctx, api, query, &output)
		if err != nil {
			return err
		}
		report, err := r.fetch(ctx, api, query, category)
		if err != nil {
			return err
		}
		accounts, err := api.Accounts(ctx, nil)
		if err != nil {
			return err
		}
		report.ResolveAccounts(accounts)
		output.Columns, output.Rows = report.Columns, report.Rows
		return o.report(command, output, report.Name, r.csv)
	})
}

func (r *reportOptions) resolveTracking(ctx context.Context, api client, query url.Values, output *reportOutput) (xero.TrackingCategory, error) {
	if len(r.tracking) == 0 && r.by == "" {
		return xero.TrackingCategory{}, nil
	}
	categories, err := api.TrackingCategories(ctx)
	if err != nil {
		return xero.TrackingCategory{}, err
	}
	for i, filter := range r.tracking {
		name, optionName, _ := strings.Cut(filter, "=")
		category, err := reportCategory(categories, name)
		if err != nil {
			return xero.TrackingCategory{}, err
		}
		var matches []xero.TrackingOption
		var names []string
		for _, option := range category.Options {
			names = append(names, option.Name)
			if strings.EqualFold(option.Name, optionName) {
				matches = append(matches, option)
			}
		}
		if len(matches) != 1 {
			return xero.TrackingCategory{}, apperr.New("invalid_argument", "option %q in %q must match exactly one option; %s", optionName, category.Name, available("options", names))
		}
		if r.name == "BalanceSheet" {
			slot := i + 1
			if r.by != "" {
				slot = 2
			}
			query.Set("trackingOptionID"+strconv.Itoa(slot), matches[0].TrackingOptionID)
		} else {
			suffix := ""
			if i > 0 || r.by != "" {
				suffix = "2"
			}
			query.Set("trackingCategoryID"+suffix, category.TrackingCategoryID)
			query.Set("trackingOptionID"+suffix, matches[0].TrackingOptionID)
		}
		output.Filters.Tracking = append(output.Filters.Tracking, reportTracking{Category: category.Name, Option: matches[0].Name})
	}
	if r.by == "" {
		return xero.TrackingCategory{}, nil
	}
	category, err := reportCategory(categories, r.by)
	if err != nil {
		return xero.TrackingCategory{}, err
	}
	output.By = &reportBy{Category: category.Name}
	if r.name == "BalanceSheet" {
		output.By.Note = "untagged balances appear only in Total; columns do not sum to Total"
	} else {
		query.Set("trackingCategoryID", category.TrackingCategoryID)
	}
	return category, nil
}

func reportCategory(categories []xero.TrackingCategory, name string) (xero.TrackingCategory, error) {
	var matches []xero.TrackingCategory
	var names []string
	for _, category := range categories {
		names = append(names, category.Name)
		if strings.EqualFold(category.Name, name) {
			matches = append(matches, category)
		}
	}
	if len(matches) != 1 {
		return xero.TrackingCategory{}, apperr.New("invalid_argument", "category %q must match exactly one category; %s", name, available("tracking categories", names))
	}
	return matches[0], nil
}

func available(noun string, names []string) string {
	if len(names) == 0 {
		return "no " + noun + " exist"
	}
	return "available: " + strings.Join(names, ", ")
}

func (r *reportOptions) fetch(ctx context.Context, api client, query url.Values, category xero.TrackingCategory) (xero.Report, error) {
	if r.name != "BalanceSheet" || r.by == "" {
		return api.Report(ctx, r.name, query)
	}
	var reports []xero.Report
	var columns []string
	for _, option := range category.Options {
		if option.Status != "ACTIVE" {
			continue
		}
		query.Set("trackingOptionID1", option.TrackingOptionID)
		report, err := api.Report(ctx, r.name, query)
		if err != nil {
			return xero.Report{}, err
		}
		report, err = report.AtDate(r.date)
		if err != nil {
			return xero.Report{}, err
		}
		reports, columns = append(reports, report), append(columns, option.Name)
	}
	query.Del("trackingOptionID1")
	total, err := api.Report(ctx, r.name, query)
	if err != nil {
		return xero.Report{}, err
	}
	total, err = total.AtDate(r.date)
	if err != nil {
		return xero.Report{}, err
	}
	return xero.MergeReportColumns(append(reports, total), append(columns, "Total"))
}
