package cmd

import "github.com/spf13/cobra"

func newAuth(o *options) *cobra.Command {
	command := &cobra.Command{Use: "auth", Short: "Inspect Custom Connection access", Args: cobra.NoArgs,
		Long: `Inspect configured Custom Connections and diagnose access problems.

` + groupHelp, RunE: func(command *cobra.Command, _ []string) error { return command.Help() }}
	command.AddCommand(newAuthStatus(o))
	return command
}
