package cmd

import "github.com/spf13/cobra"

func newManualJournal(o *options) *cobra.Command {
	command := &cobra.Command{Use: "manual-journal", Short: "Read manual journals", Args: cobra.NoArgs,
		Long: `Read manual journals in the selected organisation.

` + groupHelp,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
	command.AddCommand(newManualJournalShow(o), newManualJournalAttachments(o), newManualJournalAttachment(o))
	return command
}
