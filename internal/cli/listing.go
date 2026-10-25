package cli

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

// listingOptions holds the shared list flags; the with* fields select which
// optional flags a command offers.
type listingOptions struct {
	modifiedSince string
	page          int
	pageSize      int
	period        queryOptions
	where         string
	withPeriod    bool
	withWhere     bool
	withYear      bool
}

func (l *listingOptions) flags(command *cobra.Command) {
	flags := command.Flags()
	flags.IntVar(&l.page, "page", 0, "fetch only this page (1 or greater)")
	flags.IntVar(&l.pageSize, "page-size", 100, "items per page (1 to 1000; requires --page)")
	if l.withWhere {
		flags.StringVar(&l.where, "where", "", "additional Xero filter expression")
	}
	if l.withPeriod {
		flags.StringVar(&l.period.from, "from", "", "first accounting date (YYYY-MM-DD)")
		flags.StringVar(&l.period.to, "to", "", "last accounting date (YYYY-MM-DD)")
		flags.StringVar(&l.period.month, "month", "", "whole calendar month (YYYY-MM)")
	}
	if l.withYear {
		flags.StringVar(&l.period.year, "year", "", "whole calendar year (YYYY)")
	}
}

func (l *listingOptions) validate(command *cobra.Command) (xero.ListQuery, []string, error) {
	if command.Flags().Changed("page") && l.page < 1 {
		return xero.ListQuery{}, nil, apperr.Usage("--page must be 1 or greater")
	}
	if command.Flags().Changed("page-size") && !command.Flags().Changed("page") {
		return xero.ListQuery{}, nil, apperr.Usage("--page-size requires --page")
	}
	if l.pageSize < 1 || l.pageSize > 1000 {
		return xero.ListQuery{}, nil, apperr.Usage("--page-size must be between 1 and 1000")
	}

	query := xero.ListQuery{Page: l.page, PageSize: l.pageSize, Values: make(url.Values)}
	var clauses []string
	if l.withPeriod {
		if err := l.period.expandRange(command); err != nil {
			return xero.ListQuery{}, nil, err
		}
		from, _ := xero.ParseCalendarDate(l.period.from)
		to, _ := xero.ParseCalendarDate(l.period.to)
		clauses = append(clauses, fmt.Sprintf("Date>=DateTime(%d,%d,%d)&&Date<=DateTime(%d,%d,%d)", from.Year(), from.Month(), from.Day(), to.Year(), to.Month(), to.Day()))
	}
	if command.Flags().Changed("modified-since") {
		value, err := time.Parse(time.RFC3339, l.modifiedSince)
		if err != nil {
			value, err = time.Parse("2006-01-02T15:04:05", l.modifiedSince)
		}
		if err != nil {
			return xero.ListQuery{}, nil, apperr.New("invalid_argument", "--modified-since must be RFC 3339 or YYYY-MM-DDTHH:MM:SS")
		}
		query.ModifiedSince = value.UTC().Format("2006-01-02T15:04:05")
	}
	if command.Flags().Changed("where") && strings.TrimSpace(l.where) == "" {
		return xero.ListQuery{}, nil, apperr.New("invalid_argument", "--where must not be empty")
	}
	return query, clauses, nil
}
