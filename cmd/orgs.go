package cmd

import (
	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/spf13/cobra"
)

func newOrgs(o *options) *cobra.Command {
	return &cobra.Command{Use: "orgs", Short: "List configured organisations without connecting", Args: cobra.NoArgs,
		Long: `List configured organisation aliases and IDs, marking the effective one *.
An unset ID appears as (unverified). Names from Xero are unavailable offline.
With multiple orgs and no selection, no row is marked.

--json writes an array of {name, organisation_id, effective}. Human output
is a table on stdout; errors go to stderr. This command never reads secrets,
contacts Xero, stores tokens or requires an effective organisation.`,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			cfg, err := o.load(command)
			if err != nil {
				return err
			}
			effective, _ := cfg.Effective()
			type entry struct {
				Name           string `json:"name"`
				OrganisationID string `json:"organisation_id"`
				Effective      bool   `json:"effective"`
			}
			entries := make([]entry, 0, len(cfg.Orgs))
			var rows [][]string
			for _, name := range cfg.Names() {
				id := cfg.Orgs[name].OrganisationID
				entries = append(entries, entry{Name: name, OrganisationID: id, Effective: name == effective})
				if id == "" {
					id = "(unverified)"
				}
				mark := ""
				if name == effective {
					mark = "*"
				}
				rows = append(rows, []string{name, id, mark})
			}
			if o.json {
				return render.JSON(command.OutOrStdout(), entries)
			}
			return render.Table(command.OutOrStdout(), []string{"NAME", "ORGANISATION ID", ""}, rows, o.color)
		}),
	}
}
