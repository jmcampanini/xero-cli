package cmd

import "github.com/spf13/cobra"

func newReportTrialBalance(o *options) *cobra.Command {
	r := &reportOptions{name: "TrialBalance"}
	command := &cobra.Command{Use: "trial-balance", Short: "Read debit and credit balances at a date", Args: cobra.NoArgs,
		Long: `Read the trial balance at --date YYYY-MM-DD, defaulting to today's local
calendar date. --basis defaults to accrual; cash sends paymentsOnly=true.
Xero's debit, credit, YTD debit and YTD credit columns are kept. Revenue
and expense YTD values cover the organisation's financial year to date.

` + reportHelp,
		RunE: o.runReport(r),
	}
	command.Flags().StringVar(&r.date, "date", "", "balance date, YYYY-MM-DD; default today locally")
	command.Flags().StringVar(&r.basis, "basis", "accrual", "accounting basis: cash, accrual")
	command.Flags().BoolVar(&r.csv, "csv", false, "write CSV, with report context on stderr")
	return command
}
