package cli

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func newAccountShow(o *options) *cobra.Command {
	var date, basis string
	command := &cobra.Command{Use: "show CODE", Short: "Show an account by code, GUID or exact name", Args: cobra.ExactArgs(1),
		Long: `Find an exact account code first, including archived accounts. If no code
matches and CODE is a GUID, fetch that ID. Otherwise match the account name
case-insensitively and exactly. Ambiguous names fail with candidate codes;
an absent account is not_found. Without --date, no report lookup is made.

--date YYYY-MM-DD adds a trial-balance amount at that date. --basis defaults
to accrual; cash sends paymentsOnly=true. --basis requires --date. Asset
and expense balances are YTD debit minus credit; liability, equity and
revenue balances are YTD credit minus debit. Revenue and expense balances
cover the financial year to date. An absent row is 0.00 with a note.

Human output contains each account field. --json writes the native Xero
account object, retaining source fields and normalizing UTC timestamps.
With --date, JSON adds balance with date, basis, amount and period, plus
a note for an absent row. Report access and the Reports role are required.

` + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			if basis != "cash" && basis != "accrual" {
				return apperr.Usage("--basis must be cash or accrual")
			}
			if command.Flags().Changed("basis") && !command.Flags().Changed("date") {
				return apperr.Usage("--basis requires --date")
			}
			if command.Flags().Changed("date") {
				if _, err := xero.ParseCalendarDate(date); err != nil {
					return err
				}
			}
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			account, err := xero.ResolveAccount(command.Context(), api, args[0])
			if err != nil {
				return err
			}
			if date == "" {
				return o.object(command, account)
			}
			query := url.Values{"date": {date}}
			if basis == "cash" {
				query.Set("paymentsOnly", "true")
			}
			report, err := api.Report(command.Context(), "TrialBalance", query)
			if err != nil {
				return err
			}
			balance, err := report.BalanceForAccount(account, date, basis)
			if err != nil {
				return err
			}
			if o.json {
				raw, err := json.Marshal(account)
				if err != nil {
					return err
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(raw, &fields); err != nil {
					return err
				}
				fields["balance"], err = json.Marshal(balance)
				if err != nil {
					return err
				}
				return render.JSON(command.OutOrStdout(), fields)
			}
			if err := o.object(command, account); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(command.OutOrStdout()); err != nil {
				return err
			}
			rows := [][]string{{"balance", balance.Amount}, {"date", balance.Date}, {"basis", balance.Basis}, {"period", balance.Period}}
			if balance.Note != "" {
				rows = append(rows, []string{"note", balance.Note})
			}
			return render.Table(command.OutOrStdout(), nil, rows, o.color)
		}),
	}
	command.Flags().StringVar(&date, "date", "", "add the balance at YYYY-MM-DD")
	command.Flags().StringVar(&basis, "basis", "accrual", "cash or accrual basis for --date")
	return command
}
