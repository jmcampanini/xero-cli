package cmd

import "github.com/spf13/cobra"

func newContact(o *options) *cobra.Command {
	command := &cobra.Command{Use: "contact", Short: "Read contacts", Args: cobra.NoArgs,
		Long: `Read contacts in the selected organisation.

` + groupHelp,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
	command.AddCommand(newContactShow(o))
	return command
}
