package cli

import (
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

const reportHelp = `Human stdout has organisation, period, currency and applicable basis or
tracking context, then sectioned rows with account codes and decimal values.
--json writes one compact object with context, columns and flattened rows;
values are decimal strings. Row kind is row, summary or heading; a heading
is a titled Xero section without rows of its own and has blank values.
Native API layouts can differ from Xero's current web layouts, including
account placement, signs and section subtotals. Values are not rewritten.
--csv writes section,label,account_code and value columns to stdout;
context goes to stderr. CSV and JSON are mutually exclusive. Only human
output uses color or thousands separators. Errors go to stderr.

Reports need settings access to resolve account codes and base currency,
and report access with the Xero user's Reports role. Requests verify the
configured organisation and share the process's rate limit. Nothing prompts,
reads stdin or writes files. Redirect stdout to save JSON or CSV.`
