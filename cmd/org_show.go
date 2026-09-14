package cmd

import "github.com/spf13/cobra"

func newOrgShow(o *options) *cobra.Command {
	return &cobra.Command{Use: "show", Short: "Show organisation identity and financial settings", Args: cobra.NoArgs,
		Long: `Fetch the selected organisation's identity, locale and financial settings.
Human output is a two-column field table. --json writes the native Xero
organisation object with dates normalized.

` + readHelp,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			org, err := api.Organisation(command.Context())
			if err != nil {
				return err
			}
			return o.object(command, org)
		}),
	}
}
