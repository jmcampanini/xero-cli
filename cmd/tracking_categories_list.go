package cmd

import (
	"strconv"

	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func newTrackingCategoriesList(o *options) *cobra.Command {
	var archived, all bool
	command := &cobra.Command{Use: "list", Short: "List active or archived tracking categories", Args: cobra.NoArgs,
		Long: `List active categories by default. --archived lists only archived
categories; --all includes both. These flags are mutually exclusive.
Human columns are NAME, STATUS and the count of active OPTIONS. Xero
permits at most two active tracking categories per organisation.

` + listHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			if archived && all {
				return usage("--archived and --all are mutually exclusive")
			}
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			categories, err := api.TrackingCategories(command.Context())
			if err != nil {
				return err
			}
			items := make([]xero.TrackingCategory, 0, len(categories))
			var rows [][]string
			wanted := "ACTIVE"
			if archived {
				wanted = "ARCHIVED"
			}
			for _, category := range categories {
				if !all && category.Status != wanted {
					continue
				}
				items = append(items, category)
				count := 0
				for _, option := range category.Options {
					if option.Status == "ACTIVE" {
						count++
					}
				}
				rows = append(rows, []string{category.Name, category.Status, strconv.Itoa(count)})
			}
			return o.list(command, api.Identity(), items, []string{"NAME", "STATUS", "OPTIONS"}, rows)
		}),
	}
	command.Flags().BoolVar(&archived, "archived", false, "show only archived categories")
	command.Flags().BoolVar(&all, "all", false, "include active and archived categories")
	return command
}
