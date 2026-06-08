package radish

import "strings"

// Matcher reports whether an option label matches the current filter text, and a
// rank for ordering (lower sorts first; equal ranks keep original order).
// Components that don't reorder may ignore rank. By convention an empty filter
// matches everything.
//
// Matcher is the primary openness hook: callers inject their own to control how
// typing narrows the list, while the built-in DefaultMatcher covers the common
// fuzzy case.
type Matcher func(filter, label string) (matched bool, rank int)

// DefaultMatcher is a case-insensitive subsequence ("fuzzy") match: every rune of
// filter must appear in label, in order. Exact (case-insensitive) equality ranks
// best, then a prefix match, then everything else.
func DefaultMatcher(filter, label string) (bool, int) {
	if filter == "" {
		return true, 2
	}
	lf := strings.ToLower(filter)
	ll := strings.ToLower(label)
	if !isSubsequence(lf, ll) {
		return false, 0
	}
	switch {
	case ll == lf:
		return true, 0
	case strings.HasPrefix(ll, lf):
		return true, 1
	default:
		return true, 2
	}
}

// isSubsequence reports whether every rune of needle appears in haystack in order.
func isSubsequence(needle, haystack string) bool {
	if needle == "" {
		return true
	}
	nr := []rune(needle)
	i := 0
	for _, hc := range haystack {
		if hc == nr[i] {
			i++
			if i == len(nr) {
				return true
			}
		}
	}
	return false
}
