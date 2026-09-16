package cmd

import "github.com/spf13/cobra"

func newReportBalanceSheet(o *options) *cobra.Command {
	r := &reportOptions{name: "BalanceSheet"}
	command := &cobra.Command{Use: "balance-sheet", Short: "Read balances at a date", Args: cobra.NoArgs,
		Long: `Read the balance sheet at --date YYYY-MM-DD, defaulting to today's local
calendar date. --basis defaults to accrual; cash sends paymentsOnly=true.
--periods 1..11 and --timeframe month|quarter|year must be given together.
The date must end the selected calendar period. Xero's columns are kept.

--tracking CATEGORY=OPTION filters by exact names, ignoring case, including
archived names. Repeat at most twice for distinct categories. --by CATEGORY
runs one report per active option plus Total, sequentially, selecting only
the requested date from each response's native columns. Missing rows
are blank; untagged balances appear only in Total, so columns do not sum
to Total. A filter on the other category also applies to Total.
--by cannot accompany --periods or a filter on the same category.
--standard-layout disables custom report layouts.

` + reportHelp,
		RunE: o.runReport(r),
	}
	flags := command.Flags()
	flags.StringVar(&r.date, "date", "", "balance date, YYYY-MM-DD; default today locally")
	flags.IntVar(&r.periods, "periods", 0, "additional native comparison periods, 1..11")
	flags.StringVar(&r.timeframe, "timeframe", "", "comparison interval: month, quarter, year")
	flags.StringVar(&r.basis, "basis", "accrual", "accounting basis: cash, accrual")
	flags.StringArrayVar(&r.tracking, "tracking", nil, "filter CATEGORY=OPTION; repeat for another category")
	flags.StringVar(&r.by, "by", "", "tracking category to show as columns")
	flags.BoolVar(&r.standardLayout, "standard-layout", false, "request Xero's standard report layout")
	flags.BoolVar(&r.csv, "csv", false, "write CSV, with report context on stderr")
	return command
}
