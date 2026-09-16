package cmd

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

type listingOptions struct {
	modifiedSince string
	page          int
	pageSize      int
	period        reportOptions
	where         string
}

func (l *listingOptions) flags(command *cobra.Command, period, year, where bool) {
	flags := command.Flags()
	flags.IntVar(&l.page, "page", 0, "fetch only this page (1 or greater)")
	flags.IntVar(&l.pageSize, "page-size", 100, "items per page (1 to 1000; requires --page)")
	if where {
		flags.StringVar(&l.where, "where", "", "additional Xero filter expression")
	}
	if period {
		flags.StringVar(&l.period.from, "from", "", "first accounting date (YYYY-MM-DD)")
		flags.StringVar(&l.period.to, "to", "", "last accounting date (YYYY-MM-DD)")
		flags.StringVar(&l.period.month, "month", "", "whole calendar month (YYYY-MM)")
	}
	if year {
		flags.StringVar(&l.period.year, "year", "", "whole calendar year (YYYY)")
	}
}

func (l *listingOptions) validate(command *cobra.Command, period bool) (xero.ListQuery, []string, error) {
	if command.Flags().Changed("page") && l.page < 1 {
		return xero.ListQuery{}, nil, usage("--page must be 1 or greater")
	}
	if command.Flags().Changed("page-size") && !command.Flags().Changed("page") {
		return xero.ListQuery{}, nil, usage("--page-size requires --page")
	}
	if l.pageSize < 1 || l.pageSize > 1000 {
		return xero.ListQuery{}, nil, usage("--page-size must be between 1 and 1000")
	}
	query := xero.ListQuery{Page: l.page, PageSize: l.pageSize, Values: make(url.Values)}
	var clauses []string
	if period {
		if err := l.period.expandRange(command); err != nil {
			return query, nil, err
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
			return query, nil, apperr.New("invalid_argument", "--modified-since must be RFC 3339 or YYYY-MM-DDTHH:MM:SS")
		}
		query.ModifiedSince = value.UTC().Format("2006-01-02T15:04:05")
	}
	if command.Flags().Changed("where") && strings.TrimSpace(l.where) == "" {
		return query, nil, apperr.New("invalid_argument", "--where must not be empty")
	}
	return query, clauses, nil
}

func (l *listingOptions) filter(query *xero.ListQuery, clauses []string) {
	if l.where != "" {
		clauses = append(clauses, "("+l.where+")")
	}
	if len(clauses) > 0 {
		query.Values.Set("where", strings.Join(clauses, "&&"))
	}
}

func (o *options) records(command *cobra.Command, api client, resource string, query xero.ListQuery) error {
	result, err := api.Records(command.Context(), resource, query)
	if err != nil {
		return err
	}
	if result.Items == nil {
		result.Items = []xero.Record{}
	}
	if resource == "BankTransfers" && command.Flags().Changed("page") {
		if _, err := fmt.Fprintln(command.ErrOrStderr(), "BankTransfers is not paginated; --page and --page-size are ignored"); err != nil {
			return err
		}
	} else if query.Page != 0 {
		if _, err := fmt.Fprintf(command.ErrOrStderr(), "page %d of %d (%d items)\n", result.Page, result.PageCount, result.ItemCount); err != nil {
			return err
		}
	}
	if o.json {
		return render.JSON(command.OutOrStdout(), struct {
			Org      xero.Identity `json:"org"`
			Complete bool          `json:"complete"`
			Items    []xero.Record `json:"items"`
		}{api.Identity(), result.Complete, result.Items})
	}
	return render.Records(command.OutOrStdout(), api.Identity(), resource, result.Items, o.color)
}

func requireGUID(value string) error {
	if !guidPattern.MatchString(value) {
		return apperr.New("invalid_argument", "%q must be a GUID", value)
	}
	return nil
}

func (o *options) showRecord(command *cobra.Command, resource, id string) error {
	if err := requireGUID(id); err != nil {
		return err
	}
	api, err := o.selectedClient(command)
	if err != nil {
		return err
	}
	record, err := api.Record(command.Context(), resource, id)
	if err != nil {
		return err
	}
	if o.json {
		return render.JSON(command.OutOrStdout(), record)
	}
	attachments := record.Text("HasAttachments")
	if attachments == "true" && (resource == "BankTransactions" || resource == "ManualJournals") {
		items, countErr := api.Attachments(command.Context(), resource, id)
		if countErr == nil {
			attachments = "yes (" + strconv.Itoa(len(items)) + ")"
		} else if command.Context().Err() != nil {
			return command.Context().Err()
		}
	}
	return render.Record(command.OutOrStdout(), resource, record, attachments, o.color)
}
