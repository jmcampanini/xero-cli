package cli

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func (o *options) runReport(r *queryOptions) func(*cobra.Command, []string) error {
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
			Filters: &reportFilters{Tracking: []xero.TrackingFilter{}},
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

func (r *queryOptions) resolveTracking(ctx context.Context, api client, query url.Values, output *reportOutput) (xero.TrackingCategory, error) {
	if len(r.tracking) == 0 && r.by == "" {
		return xero.TrackingCategory{}, nil
	}
	categories, err := api.TrackingCategories(ctx)
	if err != nil {
		return xero.TrackingCategory{}, err
	}
	for i, filter := range r.tracking {
		category, option, err := xero.ResolveTrackingSelection(categories, filter)
		if err != nil {
			return xero.TrackingCategory{}, err
		}
		if r.name == "BalanceSheet" {
			slot := i + 1
			if r.by != "" {
				slot = 2
			}
			query.Set("trackingOptionID"+strconv.Itoa(slot), option.TrackingOptionID)
		} else {
			suffix := ""
			if i > 0 || r.by != "" {
				suffix = "2"
			}
			query.Set("trackingCategoryID"+suffix, category.TrackingCategoryID)
			query.Set("trackingOptionID"+suffix, option.TrackingOptionID)
		}
		output.Filters.Tracking = append(output.Filters.Tracking, xero.TrackingFilter{Category: category.Name, Option: option.Name})
	}
	if r.by == "" {
		return xero.TrackingCategory{}, nil
	}
	category, err := xero.ResolveTrackingCategory(categories, r.by)
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

func (r *queryOptions) fetch(ctx context.Context, api client, query url.Values, category xero.TrackingCategory) (xero.Report, error) {
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
		reports = append(reports, report)
		columns = append(columns, option.Name)
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
