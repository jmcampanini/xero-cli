package xero

import (
	"math/big"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// AccountBalance is the class-signed YTD trial-balance amount at an explicit date.
type AccountBalance struct {
	Amount string `json:"amount"`
	Basis  string `json:"basis"`
	Date   string `json:"date"`
	Note   string `json:"note,omitempty"`
	Period string `json:"period"`
}

// BalanceForAccount uses YTD columns, never the trial balance's period movements.
func (r Report) BalanceForAccount(account Account, date, basis string) (AccountBalance, error) {
	period, debitFirst, err := accountBalanceConvention(account.Class)
	if err != nil {
		return AccountBalance{}, err
	}
	balance := AccountBalance{Amount: "0.00", Basis: basis, Date: date, Period: period}
	debit, credit := -1, -1
	for i, column := range r.Columns {
		switch strings.ToLower(strings.Join(strings.Fields(column), " ")) {
		case "ytd debit":
			debit = i
		case "ytd credit":
			credit = i
		}
	}
	if debit < 0 || credit < 0 {
		return AccountBalance{}, apperr.New("api", "trial balance has no YTD debit and credit columns")
	}
	found := false
	for _, row := range r.Rows {
		if row.AccountID != account.AccountID {
			continue
		}
		if found || len(row.Values) <= max(debit, credit) {
			return AccountBalance{}, apperr.New("api", "trial balance has ambiguous or incomplete account rows")
		}
		found = true
		left, right := row.Values[credit], row.Values[debit]
		if debitFirst {
			left, right = right, left
		}
		amount, err := subtractDecimal(left, right)
		if err != nil {
			return AccountBalance{}, err
		}
		balance.Amount = amount
	}
	if !found {
		balance.Note = "account has no trial balance row; balance is 0.00"
	}
	return balance, nil
}

func accountBalanceConvention(class string) (period string, debitPositive bool, err error) {
	switch class {
	case "ASSET":
		return "cumulative", true, nil
	case "EXPENSE":
		return "financial_year_to_date", true, nil
	case "REVENUE":
		return "financial_year_to_date", false, nil
	case "LIABILITY", "EQUITY":
		return "cumulative", false, nil
	default:
		return "", false, apperr.New("api", "unsupported account class %q for balance", class)
	}
}

func subtractDecimal(left, right string) (string, error) {
	parse := func(value string) (*big.Int, int, error) {
		if value == "" {
			return new(big.Int), 0, nil
		}
		whole, fraction, point := strings.Cut(value, ".")
		digits := strings.TrimPrefix(strings.TrimPrefix(whole, "-"), "+") + fraction
		if whole == "" || point && fraction == "" || digits == "" {
			return nil, 0, apperr.New("api", "invalid report decimal %q", value)
		}
		for _, digit := range digits {
			if digit < '0' || digit > '9' {
				return nil, 0, apperr.New("api", "invalid report decimal %q", value)
			}
		}
		coefficient, ok := new(big.Int).SetString(whole+fraction, 10)
		if !ok {
			return nil, 0, apperr.New("api", "invalid report decimal %q", value)
		}
		return coefficient, len(fraction), nil
	}
	a, aScale, err := parse(left)
	if err != nil {
		return "", err
	}
	b, bScale, err := parse(right)
	if err != nil {
		return "", err
	}
	scale := max(2, aScale, bScale)
	power := func(n int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil) }
	a.Mul(a, power(scale-aScale))
	b.Mul(b, power(scale-bScale))
	a.Sub(a, b)
	sign := ""
	if a.Sign() < 0 {
		sign = "-"
		a.Abs(a)
	}
	digits := a.String()
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale+1-len(digits)) + digits
	}
	return sign + digits[:len(digits)-scale] + "." + digits[len(digits)-scale:], nil
}
