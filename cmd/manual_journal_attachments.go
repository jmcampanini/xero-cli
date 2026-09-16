package cmd

import "github.com/spf13/cobra"

func newManualJournalAttachments(o *options) *cobra.Command {
	return &cobra.Command{Use: "attachments ID", Short: "List document attachments", Args: cobra.ExactArgs(1),
		Long: `Inspect attachments on a manual journal.

` + attachmentsHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			return o.showAttachments(command, "ManualJournals", args[0])
		}),
	}
}
