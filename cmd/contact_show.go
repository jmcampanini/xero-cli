package cmd

import "github.com/spf13/cobra"

func newContactShow(o *options) *cobra.Command {
	return &cobra.Command{Use: "show ID", Short: "Show a saved contact", Args: cobra.ExactArgs(1),
		Long: `Inspect one contact by its Xero GUID.

` + documentShowHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			return o.showRecord(command, "Contacts", args[0])
		}),
	}
}
