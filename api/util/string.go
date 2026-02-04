package util

// ToStrings converts a slice of types which are 'strings' under the hood into
// a []string
func ToStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
