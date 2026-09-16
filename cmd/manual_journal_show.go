package cmd

import "github.com/spf13/cobra"

func newManualJournalShow(o *options) *cobra.Command {
	return &cobra.Command{Use: "show ID", Short: "Show a saved manual journal", Args: cobra.ExactArgs(1),
		Long: `Inspect one manual journal by its Xero GUID.

` + documentShowHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			return o.showRecord(command, "ManualJournals", args[0])
		}),
	}
}
