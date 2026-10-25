package reporting

import (
	"context"
	"net/url"
	"strconv"

	"github.com/jmcampanini/xero-cli/internal/xero"
)

// Client supplies report data and tracking categories.
type Client interface {
	Report(context.Context, string, url.Values) (xero.Report, error)
	TrackingCategories(context.Context) ([]xero.TrackingCategory, error)
}

// Selection identifies the report and its tracking breakdown.
type Selection struct {
	By       string
	Date     string
	Name     string
	Tracking []string
}

// ResolveTracking adds resolved filters to the query and output.
func (r Selection) ResolveTracking(ctx context.Context, api Client, query url.Values, output *Output) (xero.TrackingCategory, error) {
	if len(r.Tracking) == 0 && r.By == "" {
		return xero.TrackingCategory{}, nil
	}
	categories, err := api.TrackingCategories(ctx)
	if err != nil {
		return xero.TrackingCategory{}, err
	}
	for i, filter := range r.Tracking {
		category, option, err := xero.ResolveTrackingSelection(categories, filter)
		if err != nil {
			return xero.TrackingCategory{}, err
		}
		if r.Name == "BalanceSheet" {
			slot := i + 1
			if r.By != "" {
				slot = 2
			}
			query.Set("trackingOptionID"+strconv.Itoa(slot), option.TrackingOptionID)
		} else {
			suffix := ""
			if i > 0 || r.By != "" {
				suffix = "2"
			}
			query.Set("trackingCategoryID"+suffix, category.TrackingCategoryID)
			query.Set("trackingOptionID"+suffix, option.TrackingOptionID)
		}
		output.Filters.Tracking = append(output.Filters.Tracking, xero.TrackingFilter{Category: category.Name, Option: option.Name})
	}
	if r.By == "" {
		return xero.TrackingCategory{}, nil
	}
	category, err := xero.ResolveTrackingCategory(categories, r.By)
	if err != nil {
		return xero.TrackingCategory{}, err
	}
	output.By = &By{Category: category.Name}
	if r.Name == "BalanceSheet" {
		output.By.Note = "untagged balances appear only in Total; columns do not sum to Total"
	} else {
		query.Set("trackingCategoryID", category.TrackingCategoryID)
	}
	return category, nil
}

// Fetch retrieves a report, combining balance-sheet columns when requested.
func (r Selection) Fetch(ctx context.Context, api Client, query url.Values, category xero.TrackingCategory) (xero.Report, error) {
	if r.Name != "BalanceSheet" || r.By == "" {
		return api.Report(ctx, r.Name, query)
	}
	var reports []xero.Report
	var columns []string
	for _, option := range category.Options {
		if option.Status != "ACTIVE" {
			continue
		}
		query.Set("trackingOptionID1", option.TrackingOptionID)
		report, err := api.Report(ctx, r.Name, query)
		if err != nil {
			return xero.Report{}, err
		}
		report, err = report.AtDate(r.Date)
		if err != nil {
			return xero.Report{}, err
		}
		reports = append(reports, report)
		columns = append(columns, option.Name)
	}
	query.Del("trackingOptionID1")
	total, err := api.Report(ctx, r.Name, query)
	if err != nil {
		return xero.Report{}, err
	}
	total, err = total.AtDate(r.Date)
	if err != nil {
		return xero.Report{}, err
	}
	return xero.MergeReportColumns(append(reports, total), append(columns, "Total"))
}
