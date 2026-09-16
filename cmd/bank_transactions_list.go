package cmd

import (
	"strconv"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

func newBankTransactionsList(o *options) *cobra.Command {
	listing := listingOptions{withPeriod: true, withWhere: true}
	var account, kind, contact, reference, amount string
	var unreconciled, deleted bool
	command := &cobra.Command{Use: "list", Short: "List bank transactions for an account and period", Args: cobra.NoArgs,
		Long: `List bank transactions for --account CODE and exactly one period:
--from/--to together, or --month. Resolve the account by code, GUID or exact
name as in account show; it must be a bank account. GUIDs and names support
Xero bank accounts without codes.
Records follow Xero's date order; order within a date is unspecified.
Types are spend, receive, transfer, or all (default). Deleted records are
excluded unless --include-deleted, which marks them in human output.
--contact requires a GUID; --reference is an exact match; --amount matches
the total as a decimal. Combine these filters to check for duplicates.
--unreconciled filters saved transactions. Bank statement lines and bank
reconciliation itself are not available through this command.

` + documentListHelp + "\n\n" + readHelp,
		RunE: o.run(func(command *cobra.Command, _ []string) error {
			query, clauses, err := listing.validate(command)
			if err != nil {
				return err
			}
			if account == "" {
				return usage("--account is required")
			}
			if kind != "all" && kind != "spend" && kind != "receive" && kind != "transfer" {
				return usage("--type must be spend, receive, transfer or all")
			}
			if command.Flags().Changed("contact") {
				if err := requireGUID(contact); err != nil {
					return err
				}
			}
			if command.Flags().Changed("amount") && !xero.ValidDecimal(amount) {
				return apperr.New("invalid_argument", "--amount must be a decimal number")
			}
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			bank, err := resolveAccount(command.Context(), api, account)
			if err != nil {
				if apperr.From(err).Code == "not_found" {
					return apperr.New("invalid_argument", "bank account %q was not found", account)
				}
				return err
			}
			if bank.Type != "BANK" {
				return apperr.New("invalid_argument", "account %q is %s, not a BANK account", account, bank.Type)
			}
			clauses = append(clauses, "BankAccount.AccountID==Guid("+strconv.Quote(bank.AccountID)+")")
			if !deleted {
				clauses = append(clauses, `Status=="AUTHORISED"`)
			}
			if kind == "transfer" {
				clauses = append(clauses, `(Type=="SPEND-TRANSFER"||Type=="RECEIVE-TRANSFER")`)
			} else if kind != "all" {
				clauses = append(clauses, "Type=="+strconv.Quote(strings.ToUpper(kind)))
			}
			if contact != "" {
				clauses = append(clauses, "Contact.ContactID==Guid("+strconv.Quote(contact)+")")
			}
			if command.Flags().Changed("reference") {
				// Xero doubles embedded quotes and treats backslashes literally.
				clauses = append(clauses, `Reference=="`+strings.ReplaceAll(reference, `"`, `""`)+`"`)
			}
			if command.Flags().Changed("amount") {
				clauses = append(clauses, "Total=="+amount)
			}
			if unreconciled {
				clauses = append(clauses, "IsReconciled==false")
			}
			listing.filter(&query, clauses)
			return o.records(command, api, "BankTransactions", query)
		}),
	}
	listing.flags(command)
	command.Flags().StringVar(&account, "account", "", "required bank account code, GUID or exact name")
	command.Flags().StringVar(&kind, "type", "all", "spend, receive, transfer or all")
	command.Flags().StringVar(&contact, "contact", "", "exact contact GUID")
	command.Flags().StringVar(&reference, "reference", "", "exact reference")
	command.Flags().StringVar(&amount, "amount", "", "exact decimal total")
	command.Flags().BoolVar(&unreconciled, "unreconciled", false, "only unreconciled records")
	command.Flags().BoolVar(&deleted, "include-deleted", false, "include and mark deleted records")
	return command
}
