package cmd

import "github.com/spf13/cobra"

func newAccounts(o *options) *cobra.Command {
	command := &cobra.Command{Use: "accounts", Short: "List the chart of accounts", Args: cobra.NoArgs,
		Long: `Inspect the collection of accounts in an organisation.

` + groupHelp, RunE: func(command *cobra.Command, _ []string) error { return command.Help() }}
	command.AddCommand(newAccountsList(o))
	return command
}
