package xero

import "strings"

// ApplyFilter combines generated clauses with an optional user-supplied filter.
func ApplyFilter(query *ListQuery, clauses []string, where string) {
	if where != "" {
		clauses = append(clauses, "("+where+")")
	}
	if len(clauses) > 0 {
		query.Values.Set("where", strings.Join(clauses, "&&"))
	}
}
