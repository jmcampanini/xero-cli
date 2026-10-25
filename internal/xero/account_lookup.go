package xero

import (
	"context"
	"net/url"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// AccountLookup supplies accounts for code, GUID and name resolution.
type AccountLookup interface {
	Accounts(context.Context, url.Values) ([]Account, error)
	Account(context.Context, string) (Account, error)
}

// ResolveAccount matches an exact code, GUID, then case-insensitive name.
func ResolveAccount(ctx context.Context, api AccountLookup, operand string) (Account, error) {
	accounts, err := api.Accounts(ctx, nil)
	if err != nil {
		return Account{}, err
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
			return Account{}, err
		}
	}
	var matches []Account
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
		return Account{}, apperr.New("invalid_argument", "account name %q is ambiguous (codes: %s)", operand, strings.Join(codes, ", "))
	}
	return Account{}, apperr.New("not_found", "account %q was not found", operand)
}
