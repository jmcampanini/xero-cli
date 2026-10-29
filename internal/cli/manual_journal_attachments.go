package cli

import "github.com/spf13/cobra"

func newManualJournalAttachments(o *options) *cobra.Command {
	var output collectionOptions
	command := &cobra.Command{Use: "attachments ID", Short: "List document attachments", Args: cobra.ExactArgs(1),
		Long: `Inspect attachments on a manual journal.

` + attachmentsHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			return o.showAttachments(command, &output, "ManualJournals", args[0])
		}),
	}
	output.flags(command)
	return command
}
