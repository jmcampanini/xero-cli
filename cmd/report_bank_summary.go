package cmd

import "github.com/spf13/cobra"

func newReportBankSummary(o *options) *cobra.Command {
	r := &reportOptions{name: "BankSummary"}
	command := &cobra.Command{Use: "bank-summary", Short: "Read bank cash movements for a range", Args: cobra.NoArgs,
		Long: `Read bank opening balances, cash received, cash spent and closing balances
for --from/--to or --month YYYY-MM. Choose exactly one range. Transfers
count as cash movements. This report is not a profit and loss.

The API has no accounting-basis option for Bank Summary, so basis is
omitted from its context and JSON. Tracking filters, breakdowns and
comparison periods are not supported by this command, and its JSON has
no filters or by fields.

` + reportHelp,
		RunE: o.runReport(r),
	}
	flags := command.Flags()
	flags.StringVar(&r.from, "from", "", "first date, YYYY-MM-DD")
	flags.StringVar(&r.to, "to", "", "last date, YYYY-MM-DD")
	flags.StringVar(&r.month, "month", "", "calendar month, YYYY-MM")
	flags.BoolVar(&r.csv, "csv", false, "write CSV, with report context on stderr")
	return command
}
