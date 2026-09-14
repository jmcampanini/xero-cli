package cmd

import "github.com/spf13/cobra"

func newTrackingCategories(o *options) *cobra.Command {
	command := &cobra.Command{Use: "tracking-categories", Short: "List tracking categories", Args: cobra.NoArgs,
		Long: `Inspect the tracking categories configured in an organisation.

` + groupHelp, RunE: func(command *cobra.Command, _ []string) error { return command.Help() }}
	command.AddCommand(newTrackingCategoriesList(o))
	return command
}
