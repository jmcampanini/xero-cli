package cmd

import "github.com/spf13/cobra"

func newBankTransfers(o *options) *cobra.Command {
	command := &cobra.Command{Use: "bank-transfers", Short: "Read bank transfers", Args: cobra.NoArgs,
		Long: `Read bank transfers in the selected organisation.

` + groupHelp,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
	command.AddCommand(newBankTransfersList(o))
	return command
}
