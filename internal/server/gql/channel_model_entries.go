package gql

import (
	"slices"
	"strings"

	"github.com/looplj/axonhub/internal/server/biz"
)

// sortChannelModelEntries returns a new slice with the entries sorted by
// RequestModel in ascending order. The sort is stable, so entries sharing the
// same RequestModel keep their relative input order, making the output
// deterministic for a given input.
func sortChannelModelEntries(entries []biz.ChannelModelEntry) []biz.ChannelModelEntry {
	sorted := slices.Clone(entries)
	slices.SortStableFunc(sorted, func(a, b biz.ChannelModelEntry) int {
		return strings.Compare(a.RequestModel, b.RequestModel)
	})

	return sorted
}
