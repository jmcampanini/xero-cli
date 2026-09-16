package cmd

import "github.com/spf13/cobra"

func newManualJournalAttachment(o *options) *cobra.Command {
	var output string
	command := &cobra.Command{Use: "attachment ID FILENAME", Short: "Download one document attachment", Args: cobra.ExactArgs(2),
		Long: `Read an attachment from a manual journal.

` + attachmentHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			return o.downloadAttachment(command, "ManualJournals", args[0], args[1], output)
		}),
	}
	command.Flags().StringVar(&output, "output", "", "destination path, or - for stdout")
	return command
}
