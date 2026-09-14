package cmd

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newAccountsList(o *options) *cobra.Command {
	var accountType, class, status string
	var bank bool
	command := &cobra.Command{Use: "list", Short: "List accounts with optional type and status filters", Args: cobra.NoArgs,
		Long: `List accounts in code order. Status defaults to active; all omits the
status filter. Type and class use Xero enums, uppercased before comparison.
--bank is equivalent to --type BANK; a different --type conflicts with it.
Human columns are CODE, NAME, TYPE, TAX, STATUS and bank CURRENCY.

` + listHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			if status != "active" && status != "archived" && status != "all" {
				return usage("--status must be active, archived or all")
			}
			if bank && accountType != "" && !strings.EqualFold(accountType, "BANK") {
				return usage("--bank conflicts with --type %s", accountType)
			}
			filters := []string{}
			kind := strings.ToUpper(accountType)
			if bank {
				kind = "BANK"
			}
			if kind != "" {
				filters = append(filters, "Type=="+strconv.Quote(kind))
			}
			if class != "" {
				filters = append(filters, "Class=="+strconv.Quote(strings.ToUpper(class)))
			}
			if status != "all" {
				filters = append(filters, "Status=="+strconv.Quote(strings.ToUpper(status)))
			}
			query := url.Values{"order": {"Code"}}
			if len(filters) > 0 {
				query.Set("where", strings.Join(filters, " AND "))
			}
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			accounts, err := api.Accounts(command.Context(), query)
			if err != nil {
				return err
			}
			var rows [][]string
			for _, account := range accounts {
				currency := ""
				if account.Type == "BANK" {
					currency = account.CurrencyCode
				}
				rows = append(rows, []string{account.Code, account.Name, account.Type, account.TaxType, account.Status, currency})
			}
			return o.list(command, api.Identity(), accounts, []string{"CODE", "NAME", "TYPE", "TAX", "STATUS", "CURRENCY"}, rows)
		}),
	}
	command.Flags().StringVar(&accountType, "type", "", "Xero account type")
	command.Flags().StringVar(&class, "class", "", "Xero account class")
	command.Flags().StringVar(&status, "status", "active", "active, archived or all")
	command.Flags().BoolVar(&bank, "bank", false, "show bank accounts")
	return command
}
