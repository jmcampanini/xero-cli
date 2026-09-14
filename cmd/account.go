package cmd

import "github.com/spf13/cobra"

func newAccount(o *options) *cobra.Command {
	command := &cobra.Command{Use: "account", Short: "Inspect one account", Args: cobra.NoArgs,
		Long: `Inspect one chart-of-accounts record by code, GUID or exact name.

` + groupHelp, RunE: func(command *cobra.Command, _ []string) error { return command.Help() }}
	command.AddCommand(newAccountShow(o))
	return command
}
