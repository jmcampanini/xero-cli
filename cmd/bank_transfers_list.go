package cmd

import "github.com/spf13/cobra"

func newBankTransfersList(o *options) *cobra.Command {
	listing := listingOptions{withPeriod: true}
	command := &cobra.Command{Use: "list", Short: "List transfers between bank accounts", Args: cobra.NoArgs,
		Long: `List bank transfers for --from/--to together or --month. Each row shows
both bank accounts, amount, reference and both reconciliation flags.
Xero does not paginate this endpoint. --page and --page-size are accepted
and validated, then ignored with a note on stderr; complete stays true.

` + documentListHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			query, clauses, err := listing.validate(command)
			if err != nil {
				return err
			}
			listing.filter(&query, clauses)
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			return o.records(command, api, "BankTransfers", query)
		}),
	}
	listing.flags(command)
	return command
}
