package cli

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func (r *reportOptions) validate(command *cobra.Command, jsonOutput bool, now time.Time) (url.Values, error) {
	if r.csv && jsonOutput {
		return nil, apperr.Usage("--csv and --json cannot be combined")
	}
	if r.name != "BankSummary" && r.basis != "cash" && r.basis != "accrual" {
		return nil, apperr.Usage("--basis must be cash or accrual")
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

func (r *reportOptions) validateComparisons(command *cobra.Command) error {
	flags := command.Flags()
	if flags.Changed("periods") != flags.Changed("timeframe") {
		return apperr.Usage("--periods and --timeframe must be given together")
	}
	if !flags.Changed("periods") {
		return nil
	}
	if r.periods < 1 || r.periods > 11 {
		return apperr.Usage("--periods must be between 1 and 11")
	}
	if r.timeframe != "month" && r.timeframe != "quarter" && r.timeframe != "year" {
		return apperr.Usage("--timeframe must be month, quarter or year")
	}
	if flags.Changed("by") {
		return apperr.Usage("--by and --periods cannot be combined")
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
