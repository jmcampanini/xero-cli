package cmd

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newManualJournalsList(o *options) *cobra.Command {
	listing := listingOptions{withPeriod: true, withWhere: true, withYear: true}
	var status string
	command := &cobra.Command{Use: "list", Short: "List manual journals for a period", Args: cobra.NoArgs,
		Long: `List journals for --from/--to together, --month, or --year. Status defaults
to posted; all includes draft, voided, deleted and archived journals.
Records follow Xero's date order; order within a date is unspecified.
DEBITS sums positive journal line amounts exactly. Cash-basis treatment
comes from ShowOnCashBasisReports. --modified-since accepts RFC 3339 or
YYYY-MM-DDTHH:MM:SS (UTC) and sends If-Modified-Since in UTC.

` + documentListHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			query, clauses, err := listing.validate(command)
			if err != nil {
				return err
			}
			if status != "all" && status != "draft" && status != "posted" && status != "voided" {
				return usage("--status must be draft, posted, voided or all")
			}
			if status != "all" {
				clauses = append(clauses, "Status=="+strconv.Quote(strings.ToUpper(status)))
			}
			listing.filter(&query, clauses)
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			return o.records(command, api, "ManualJournals", query)
		}),
	}
	listing.flags(command)
	command.Flags().StringVar(&status, "status", "posted", "draft, posted, voided or all")
	command.Flags().StringVar(&listing.modifiedSince, "modified-since", "", "only records modified since this timestamp")
	return command
}
