package bucket

import (
	"maps"
	"slices"
	"strings"
)

// stringSet is a collection of unique strings, implemented as a map with empty struct values.
type stringSet map[string]struct{}

// Add adds a string to the set.
func (s stringSet) Add(v string) { s[v] = struct{}{} }

// Del removes a string from the set.
func (s stringSet) Del(v string) { delete(s, v) }

// Has returns true if the string exists in the set.
func (s stringSet) Has(v string) bool { _, ok := s[v]; return ok }

// mapKeysSorted returns all keys of the given map in lexicographically sorted order.
// It first collects the map keys into a slice and then sorts them deterministically.
// Useful when iteration over map keys must be stable (e.g. for output, tests, or hashing).
func mapKeysSorted[M ~map[string]V, V any](m M) []string {
	keys := slices.Collect(maps.Keys(m)) // iterator -> []string
	slices.Sort(keys)
	return keys
}

// getOrInitSet retrieves the [stringSet] for the given key from the map.
// If the key does not exist, it initializes a new [stringSet], stores it in the map, and returns it.
func getOrInitSet(m map[string]stringSet, key string) stringSet {
	if set, ok := m[key]; ok {
		return set
	}
	set := make(stringSet)
	m[key] = set
	return set
}

// setFromSliceTrimmed creates a [stringSet] from a slice of strings.
// It trims leading and trailing whitespace from each string and ignores empty results.
func setFromSliceTrimmed(in []string) stringSet {
	s := make(stringSet, len(in))
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			s.Add(v)
		}
	}
	return s
}
