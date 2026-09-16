package xero

import (
	"math/big"
	"strings"

	"github.com/jmcampanini/xero-cli/internal/apperr"
)

// ValidDecimal reports whether text is an ordinary signed decimal, without exponents.
func ValidDecimal(text string) bool {
	text = strings.TrimPrefix(text, "-")
	whole, fraction, dot := strings.Cut(text, ".")
	if whole == "" || dot && fraction == "" {
		return false
	}
	for _, digit := range whole + fraction {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// JournalDebits sums positive journal line amounts exactly, retaining decimal scale.
func JournalDebits(lines []Record) (string, error) {
	total := new(big.Rat)
	scale := 2
	for _, line := range lines {
		text := line.Text("LineAmount")
		if text == "" && line.Text("IsBlank") == "true" {
			continue
		}
		if !ValidDecimal(text) {
			return "", apperr.New("api", "journal line has no valid LineAmount")
		}
		value, _ := new(big.Rat).SetString(text)
		if value.Sign() > 0 {
			total.Add(total, value)
			_, fraction, _ := strings.Cut(text, ".")
			scale = max(scale, len(fraction))
		}
	}
	return total.FloatString(scale), nil
}
