package cmd

import "github.com/spf13/cobra"

func newBankTransaction(o *options) *cobra.Command {
	command := &cobra.Command{Use: "bank-transaction", Short: "Read bank transactions", Args: cobra.NoArgs,
		Long: `Read bank transactions in the selected organisation.

` + groupHelp,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
	command.AddCommand(newBankTransactionShow(o), newBankTransactionAttachments(o), newBankTransactionAttachment(o))
	return command
}
