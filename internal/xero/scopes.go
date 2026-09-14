package xero

import "strings"

// Capabilities maps token scopes to command-group availability.
// Read scopes enable reads; non-read scopes enable both reads and writes.
func Capabilities(scopes []string) map[string]bool {
	has := func(names ...string) bool {
		for _, scope := range scopes {
			for _, name := range names {
				if scope == name {
					return true
				}
			}
		}
		return false
	}
	reports := has("accounting.reports.read")
	for _, scope := range scopes {
		if strings.HasPrefix(scope, "accounting.reports.") && strings.HasSuffix(scope, ".read") {
			reports = true
		}
	}
	return map[string]bool{
		"accounts":            has("accounting.settings", "accounting.settings.read"),
		"tracking":            has("accounting.settings", "accounting.settings.read"),
		"reports":             reports,
		"transactions":        has("accounting.transactions", "accounting.transactions.read") || has("accounting.banktransactions", "accounting.banktransactions.read") && has("accounting.manualjournals", "accounting.manualjournals.read"),
		"contacts":            has("accounting.contacts", "accounting.contacts.read"),
		"accounts writes":     has("accounting.settings"),
		"tracking writes":     has("accounting.settings"),
		"transactions writes": has("accounting.transactions") || has("accounting.banktransactions") && has("accounting.manualjournals"),
		"contacts writes":     has("accounting.contacts"),
	}
}
