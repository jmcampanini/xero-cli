package cmd

import "github.com/spf13/cobra"

func newBankTransactionShow(o *options) *cobra.Command {
	return &cobra.Command{Use: "show ID", Short: "Show a saved bank transaction", Args: cobra.ExactArgs(1),
		Long: `Inspect one bank transaction by its Xero GUID.

` + documentShowHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			return o.showRecord(command, "BankTransactions", args[0])
		}),
	}
}
