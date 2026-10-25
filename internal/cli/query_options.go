package cli

import (
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

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

func (r *reportOptions) expandRange(command *cobra.Command) error {
	flags := command.Flags()
	if flags.Changed("from") != flags.Changed("to") {
		return apperr.Usage("--from and --to must be given together")
	}
	forms := 0
	for _, name := range []string{"from", "month", "year"} {
		if flags.Changed(name) {
			forms++
		}
	}
	if forms != 1 {
		return apperr.Usage("choose exactly one period: --from/--to, --month, or --year where supported")
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

func (r *reportOptions) validateTracking(command *cobra.Command) error {
	if r.by != "" && len(r.tracking) > 1 {
		return apperr.Usage("--by allows at most one --tracking filter on the other category")
	}
	if len(r.tracking) > 2 {
		return apperr.Usage("--tracking may be given at most twice")
	}
	if command.Flags().Changed("by") && strings.TrimSpace(r.by) == "" {
		return apperr.Usage("--by requires a category name")
	}
	var categories []string
	for _, filter := range r.tracking {
		category, option, found := strings.Cut(filter, "=")
		if !found || strings.TrimSpace(category) == "" || strings.TrimSpace(option) == "" {
			return apperr.Usage("--tracking must be CATEGORY=OPTION")
		}
		for _, previous := range categories {
			if strings.EqualFold(previous, category) {
				return apperr.Usage("--tracking cannot repeat category %q", category)
			}
		}
		if r.by != "" && strings.EqualFold(r.by, category) {
			return apperr.Usage("--by and --tracking cannot use the same category")
		}
		categories = append(categories, category)
	}
	return nil
}
