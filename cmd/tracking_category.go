package cmd

import "github.com/spf13/cobra"

func newTrackingCategory(o *options) *cobra.Command {
	command := &cobra.Command{Use: "tracking-category", Short: "Inspect one tracking category", Args: cobra.NoArgs,
		Long: `Inspect a tracking category and all of its options.

` + groupHelp, RunE: func(command *cobra.Command, _ []string) error { return command.Help() }}
	command.AddCommand(newTrackingCategoryShow(o))
	return command
}
