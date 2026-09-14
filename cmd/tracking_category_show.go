package cmd

import (
	"fmt"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/render"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func newTrackingCategoryShow(o *options) *cobra.Command {
	return &cobra.Command{Use: "show NAME", Short: "Show a tracking category and all options", Args: cobra.ExactArgs(1),
		Long: `Match NAME exactly, ignoring case, across active and archived categories.
An absent category is not_found; multiple matches are invalid_argument.
Print the category header and every option's NAME and STATUS, including
archived options. --json writes the native object with its Options array.
Categories and options are discovered per organisation, never assumed.

` + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			categories, err := api.TrackingCategories(command.Context())
			if err != nil {
				return err
			}
			var matches []xero.TrackingCategory
			for _, category := range categories {
				if strings.EqualFold(category.Name, args[0]) {
					matches = append(matches, category)
				}
			}
			if len(matches) == 0 {
				return apperr.New("not_found", "tracking category %q was not found", args[0])
			}
			if len(matches) > 1 {
				return apperr.New("invalid_argument", "tracking category %q is ambiguous", args[0])
			}
			category := matches[0]
			if o.json {
				return render.JSON(command.OutOrStdout(), category)
			}
			if _, err := fmt.Fprintf(command.OutOrStdout(), "%s  %s  (%s)\n", category.Name, category.Status, category.TrackingCategoryID); err != nil {
				return err
			}
			var rows [][]string
			for _, option := range category.Options {
				rows = append(rows, []string{option.Name, option.Status})
			}
			return render.Table(command.OutOrStdout(), []string{"NAME", "STATUS"}, rows, o.color)
		}),
	}
}
