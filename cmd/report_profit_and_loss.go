package cmd

import "github.com/spf13/cobra"

func newReportProfitAndLoss(o *options) *cobra.Command {
	r := &reportOptions{name: "ProfitAndLoss"}
	command := &cobra.Command{Use: "profit-and-loss", Short: "Read profit and loss for an exact range", Args: cobra.NoArgs,
		Long: `Read profit and loss for --from/--to, --month YYYY-MM or --year YYYY.
Choose exactly one range. --basis defaults to accrual; cash sends
paymentsOnly=true. A cash profit and loss is not a cash-flow statement.

--periods 1..11 and --timeframe month|quarter|year must be given together.
Ranges must cover whole calendar periods. Xero applies the supplied range
to each comparison; it does not split a year into months. A 30-day base
month can truncate earlier 31-day months. All returned columns are kept.

--tracking CATEGORY=OPTION filters by exact names, ignoring case, including
archived names. Repeat at most twice for distinct categories. --by CATEGORY
uses Xero's native option columns. It cannot accompany --periods or a filter
on the same category. --standard-layout disables custom report layouts.

` + reportHelp,
		Example: `  xero report profit-and-loss --month 2025-12
  xero report profit-and-loss --month 2025-12 --periods 11 --timeframe month
  xero report profit-and-loss --year 2025 --by Property --csv`,
		RunE: o.runReport(r),
	}
	flags := command.Flags()
	flags.StringVar(&r.from, "from", "", "first date, YYYY-MM-DD")
	flags.StringVar(&r.to, "to", "", "last date, YYYY-MM-DD")
	flags.StringVar(&r.month, "month", "", "calendar month, YYYY-MM")
	flags.StringVar(&r.year, "year", "", "calendar year, YYYY")
	flags.IntVar(&r.periods, "periods", 0, "additional native comparison periods, 1..11")
	flags.StringVar(&r.timeframe, "timeframe", "", "comparison interval: month, quarter, year")
	flags.StringVar(&r.basis, "basis", "accrual", "accounting basis: cash, accrual")
	flags.StringArrayVar(&r.tracking, "tracking", nil, "filter CATEGORY=OPTION; repeat for another category")
	flags.StringVar(&r.by, "by", "", "tracking category to show as columns")
	flags.BoolVar(&r.standardLayout, "standard-layout", false, "request Xero's standard report layout")
	flags.BoolVar(&r.csv, "csv", false, "write CSV, with report context on stderr")
	return command
}
