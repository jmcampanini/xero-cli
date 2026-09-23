package cmd

import (
	"time"

	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func newAccountTransactions(o *options) *cobra.Command {
	var period reportOptions
	var deleted, csv bool
	command := &cobra.Command{Use: "transactions CODE", Short: "Show supported cash movements on an account", Args: cobra.ExactArgs(1),
		Long: `Assemble cash-basis movements for an account resolved by code, GUID or exact
name. Choose --from/--to together or --month. Only the organisation's base
currency is supported; relevant foreign-currency activity fails explicitly.
There is no --basis flag. Nothing prompts or changes accounting records.

Bank accounts show Spend/Receive Money and deduplicated bank transfers, one
row per document. Other accounts show matching bank-transaction lines and
posted manual-journal lines included on cash-basis reports. Line postings
are net of tax. Dates are UTC calendar dates; amounts use exact decimals.
Rows sort by date, source type, then source ID, keeping document line order.
Deleted/voided rows require --include-deleted and never affect calculations.

Unfiltered output starts with the prior day's cash trial-balance amount.
Asset/expense balances are debit-positive; liability/equity/revenue balances
are credit-positive. Income/expense balances are financial-year-to-date,
zero at the financial-year start, and require a range within one financial
year. Other balances are cumulative. A missing account row means 0.00.
The closing balance is reconstructed from the supported rows. For ranges
ending today or earlier, a cash trial-balance comparison shows unexplained
as trial balance minus reconstructed closing. Future ranges omit it.

--tracking CATEGORY=OPTION accepts up to two different categories, resolved
by exact names ignoring case, including archived options. Both filters must
match. It shows matching rows and signed net movement only, without opening,
running or closing balances and without trial-balance requests. Bank rows
match their combined tracking and retain the whole document amount; they
are not allocations among options. Transfers use the selected side's
tracking when returned. Movement-only ranges may cross financial years.

Coverage is always incomplete, including for bank accounts. Invoice/bill
payments and refunds, prepayment/overpayment allocations and refunds, and
system-generated lines (tax, FX, payroll) are excluded. A zero unexplained
difference does not prove completeness or identify the cause of omissions.

Human output shows context, rows, totals and coverage. --json writes one
compact object plus newline, exact decimal strings, complete:false and gap.
Source tracking keeps native field names and unknown fields. --csv writes
rows to stdout with full source IDs and separate type, subtype and status
columns; context, totals and coverage go to stderr. --csv and --json conflict.
Filtered output omits balance fields and the balance column. CSV has no
color or grouped numbers. All reads finish before any payload is written.

` + readHelp,
		Example: `  xero account transactions 090 --month 2025-07
  xero account transactions 429 --from 2025-01-01 --to 2025-12-31 --json
  xero account transactions 429 --month 2025-07 --tracking Region=North --csv`,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			if csv && o.json {
				return usage("--csv and --json cannot be combined")
			}
			if err := period.expandRange(command); err != nil {
				return err
			}
			if err := period.validateTracking(command); err != nil {
				return err
			}
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			ctx := command.Context()
			account, err := resolveAccount(ctx, api, args[0])
			if err != nil {
				return err
			}
			org, err := api.Organisation(ctx)
			if err != nil {
				return err
			}
			var filters []xero.TrackingFilter
			if len(period.tracking) > 0 {
				categories, err := api.TrackingCategories(ctx)
				if err != nil {
					return err
				}
				for _, filter := range period.tracking {
					category, option, err := resolveTrackingSelection(categories, filter)
					if err != nil {
						return err
					}
					filters = append(filters, xero.TrackingFilter{Category: category.Name, Option: option.Name})
				}
			}
			result, err := xero.ReadAccountTransactions(ctx, api, xero.TransactionQuery{Account: account, From: period.from, IncludeDeleted: deleted, Organisation: org, To: period.to, Today: time.Now().Format("2006-01-02"), Tracking: filters})
			if err != nil {
				return err
			}
			if o.json {
				return render.JSON(command.OutOrStdout(), result)
			}
			return render.Transactions(command.OutOrStdout(), command.ErrOrStderr(), result, csv, o.color)
		})}
	command.Flags().StringVar(&period.from, "from", "", "first accounting date (YYYY-MM-DD)")
	command.Flags().StringVar(&period.to, "to", "", "last accounting date (YYYY-MM-DD)")
	command.Flags().StringVar(&period.month, "month", "", "whole calendar month (YYYY-MM)")
	command.Flags().StringArrayVar(&period.tracking, "tracking", nil, "matching CATEGORY=OPTION; at most two categories")
	command.Flags().BoolVar(&deleted, "include-deleted", false, "show deleted/voided rows without counting them")
	command.Flags().BoolVar(&csv, "csv", false, "write CSV rows; context and totals go to stderr")
	return command
}
