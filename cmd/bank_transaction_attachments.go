package cmd

import "github.com/spf13/cobra"

func newBankTransactionAttachments(o *options) *cobra.Command {
	return &cobra.Command{Use: "attachments ID", Short: "List document attachments", Args: cobra.ExactArgs(1),
		Long: `Inspect attachments on a bank transaction.

` + attachmentsHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			return o.showAttachments(command, "BankTransactions", args[0])
		}),
	}
}
