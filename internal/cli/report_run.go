package cli

import (
	"time"

	"github.com/jmcampanini/xero-cli/internal/reporting"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

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
		output := reporting.Output{Basis: r.basis, Currency: org.BaseCurrency, Date: r.date, From: r.from,
			Org: xero.Identity{Name: org.Name, ID: org.OrganisationID}, Report: r.name, To: r.to,
			Filters: &reporting.Filters{Tracking: []xero.TrackingFilter{}},
		}
		selection := reporting.Selection{By: r.by, Date: r.date, Name: r.name, Tracking: r.tracking}
		category, err := selection.ResolveTracking(ctx, api, query, &output)
		if err != nil {
			return err
		}
		report, err := selection.Fetch(ctx, api, query, category)
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
