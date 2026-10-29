package cli

import (
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func newBankTransfersList(o *options) *cobra.Command {
	var output collectionOptions
	listing := listingOptions{withPeriod: true}
	command := &cobra.Command{Use: "list", Short: "List transfers between bank accounts", Args: cobra.NoArgs,
		Long: `List bank transfers for --from/--to together or --month. Each row shows
both bank accounts, amount, reference and both reconciliation flags.
Records order by date then ID.
Xero does not paginate this endpoint. --page and --page-size are accepted
and validated, then ignored with a note on stderr; complete stays true.
Machine records use the Xero Accounting API BankTransfers resource schema.

` + documentListHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			if err := output.validate(command, o.json); err != nil {
				return err
			}
			query, clauses, err := listing.validate(command)
			if err != nil {
				return err
			}
			xero.ApplyFilter(&query, clauses, listing.where)
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			return o.records(command, &output, api, "BankTransfers", query)
		}),
	}
	listing.flags(command)
	output.flags(command)
	return command
}
