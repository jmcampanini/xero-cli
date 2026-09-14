package cmd

import (
	"context"
	"regexp"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/apperr"
	"github.com/jmcampanini/xero-cli/internal/xero"
	"github.com/spf13/cobra"
)

var guidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func newAccountShow(o *options) *cobra.Command {
	return &cobra.Command{Use: "show CODE", Short: "Show an account by code, GUID or exact name", Args: cobra.ExactArgs(1),
		Long: `Find an exact account code first, including archived accounts. If no code
matches and CODE is a GUID, fetch that ID. Otherwise match the account name
case-insensitively and exactly. Ambiguous names fail with candidate codes;
an absent account is not_found. No balance or report lookup is performed.

Human output contains each account field. --json writes the native Xero
account object, retaining source fields and normalizing UTC timestamps.

` + readHelp,
		RunE: o.run(func(command *cobra.Command, args []string) error {
			api, err := o.selectedClient(command)
			if err != nil {
				return err
			}
			account, err := resolveAccount(command.Context(), api, args[0])
			if err != nil {
				return err
			}
			return o.object(command, account)
		}),
	}
}

func resolveAccount(ctx context.Context, api client, operand string) (xero.Account, error) {
	accounts, err := api.Accounts(ctx, nil)
	if err != nil {
		return xero.Account{}, err
	}
	for _, account := range accounts {
		if account.Code == operand {
			return account, nil
		}
	}
	if guidPattern.MatchString(operand) {
		account, err := api.Account(ctx, operand)
		if err == nil {
			return account, nil
		}
		if apperr.From(err).Code != "not_found" {
			return xero.Account{}, err
		}
	}
	var matches []xero.Account
	var codes []string
	for _, account := range accounts {
		if strings.EqualFold(account.Name, operand) {
			matches = append(matches, account)
			codes = append(codes, account.Code)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return xero.Account{}, apperr.New("invalid_argument", "account name %q is ambiguous (codes: %s)", operand, strings.Join(codes, ", "))
	}
	return xero.Account{}, apperr.New("not_found", "account %q was not found", operand)
}
