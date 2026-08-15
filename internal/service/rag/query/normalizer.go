// Package query contains query preparation that is independent from search-engine tokenization.
package query

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Normalize applies compatibility normalization and stable whitespace/case rules.
// Tokenization is deliberately left to ParadeDB pg_search.
func Normalize(text string) string {
	text = norm.NFKC.String(text)
	text = strings.ToLower(text)
	return strings.Join(strings.Fields(text), " ")
}
