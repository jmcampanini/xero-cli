package cmd

import "github.com/spf13/cobra"

func newContacts(o *options) *cobra.Command {
	command := &cobra.Command{Use: "contacts", Short: "Read contacts", Args: cobra.NoArgs,
		Long: `Read contacts in the selected organisation.

` + groupHelp,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
	command.AddCommand(newContactsList(o))
	return command
}
