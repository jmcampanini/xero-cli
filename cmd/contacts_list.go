package cmd

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newContactsList(o *options) *cobra.Command {
	var listing listingOptions
	var search, status string
	var customers, suppliers bool
	command := &cobra.Command{Use: "list", Short: "Find contacts and their IDs", Args: cobra.NoArgs,
		Long: `List contacts in name then ID order. --search matches Xero's contact name,
first and last name, email and contact number search. Status defaults to
active; archived includes only archived contacts; all includes both.
--customers and --suppliers together require both roles. Full paged records
are requested because Xero's summaryOnly omits those roles and their filters.

` + documentListHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			query, clauses, err := listing.validate(command, false)
			if err != nil {
				return err
			}
			if status != "active" && status != "archived" && status != "all" {
				return usage("--status must be active, archived or all")
			}
			query.Values.Set("includeArchived", strconv.FormatBool(status != "active"))
			if command.Flags().Changed("search") {
				query.Values.Set("searchTerm", search)
			}
			if status != "all" {
				clauses = append(clauses, "ContactStatus=="+strconv.Quote(strings.ToUpper(status)))
			}
			if customers {
				clauses = append(clauses, "IsCustomer==true")
			}
			if suppliers {
				clauses = append(clauses, "IsSupplier==true")
			}
			listing.filter(&query, clauses)
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			return o.records(command, api, "Contacts", query)
		}),
	}
	listing.flags(command, false, false, true)
	command.Flags().StringVar(&search, "search", "", "Xero contact search text")
	command.Flags().StringVar(&status, "status", "active", "active, archived or all")
	command.Flags().BoolVar(&customers, "customers", false, "only customers")
	command.Flags().BoolVar(&suppliers, "suppliers", false, "only suppliers")
	return command
}
