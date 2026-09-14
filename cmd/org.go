package cmd

import "github.com/spf13/cobra"

func newOrg(o *options) *cobra.Command {
	command := &cobra.Command{Use: "org", Short: "Inspect the selected organisation", Args: cobra.NoArgs,
		Long: `Inspect the selected organisation's identity and financial settings.

` + groupHelp, RunE: func(command *cobra.Command, _ []string) error { return command.Help() }}
	command.AddCommand(newOrgShow(o))
	return command
}
