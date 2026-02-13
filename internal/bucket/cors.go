package bucket

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/cli"
)

const (
	corsKeyOrigins         = "origins"
	corsKeyResponseHeaders = "response-headers"
	corsKeyMaxAge          = "max-age"
)

// PatchCORS applies additive (--cors) and subtractive (--delete-cors) updates to an
// existing CORS configuration. Semantics for add:
//   - origins/response-headers: union (trimmed, deduped, sorted)
//   - max-age: set only if explicitly provided
//
// Removals:
//   - origins/response-headers: set-difference
//   - specifying max-age here is rejected
//   - if origins are targeted and removal leaves zero origins, the CORS
//     configuration is cleared (function returns nil, changed=true)
func PatchCORS(base *storage.CORSConfig, addChunks []string, removeChunks []string) (*storage.CORSConfig, error) {
	if len(addChunks) == 0 && len(removeChunks) == 0 {
		return base, nil
	}

	work := &storage.CORSConfig{}
	if base != nil {
		work.Origins = append([]string(nil), base.Origins...)
		work.ResponseHeaders = append([]string(nil), base.ResponseHeaders...)
		work.MaxAge = base.MaxAge
	}

	if len(addChunks) > 0 {
		addSpec, addMask, err := parseCORSLooseWithMask(addChunks)
		if err != nil {
			return nil, fmt.Errorf("add cors: %w", err)
		}

		if addMask.Origins {
			union := make(map[string]struct{}, len(work.Origins)+len(addSpec.Origins))
			for _, o := range work.Origins {
				union[o] = struct{}{}
			}
			for _, o := range addSpec.Origins {
				union[o] = struct{}{}
			}
			work.Origins = make([]string, 0, len(union))
			for o := range union {
				work.Origins = append(work.Origins, o)
			}
			sort.Strings(work.Origins)
		}
		if addMask.ResponseHeaders {
			union := make(map[string]struct{}, len(work.ResponseHeaders)+len(addSpec.ResponseHeaders))
			for _, h := range work.ResponseHeaders {
				union[h] = struct{}{}
			}
			for _, h := range addSpec.ResponseHeaders {
				union[h] = struct{}{}
			}
			work.ResponseHeaders = make([]string, 0, len(union))
			for h := range union {
				work.ResponseHeaders = append(work.ResponseHeaders, h)
			}
			sort.Strings(work.ResponseHeaders)
		}
		if addMask.MaxAge {
			work.MaxAge = addSpec.MaxAge
		}
	}

	var removedOrigins bool
	if len(removeChunks) > 0 {
		rmSpec, rmMask, err := parseCORSLooseWithMask(removeChunks)
		if err != nil {
			return nil, fmt.Errorf("delete cors: %w", err)
		}
		if rmMask.MaxAge {
			return nil, fmt.Errorf("delete-cors: %s cannot be specified; use --cors to set it", corsKeyMaxAge)
		}

		if rmMask.Origins && len(rmSpec.Origins) > 0 {
			removedOrigins = true
			rm := make(map[string]struct{}, len(rmSpec.Origins))
			for _, o := range rmSpec.Origins {
				rm[o] = struct{}{}
			}
			kept := make([]string, 0, len(work.Origins))
			for _, o := range work.Origins {
				if _, found := rm[o]; !found {
					kept = append(kept, o)
				}
			}
			work.Origins = kept
		}

		if rmMask.ResponseHeaders && len(rmSpec.ResponseHeaders) > 0 {
			rm := make(map[string]struct{}, len(rmSpec.ResponseHeaders))
			for _, h := range rmSpec.ResponseHeaders {
				rm[h] = struct{}{}
			}
			kept := make([]string, 0, len(work.ResponseHeaders))
			for _, h := range work.ResponseHeaders {
				if _, found := rm[h]; !found {
					kept = append(kept, h)
				}
			}
			work.ResponseHeaders = kept
		}

		// If delete-cors targeted origins and removal resulted in zero origins,
		// interpret as "clear CORS" (drop the config entirely).
		if removedOrigins && len(work.Origins) == 0 {
			return nil, nil
		}
	}

	return work, nil
}

// corsFieldMask indicates which keys were explicitly present across chunks.
type corsFieldMask struct {
	Origins         bool
	ResponseHeaders bool
	MaxAge          bool
}

// parseCORSLooseWithMask parses and merges CORS chunks in a tolerant way,
// intended for UPDATE flows (add/remove) rather than CREATE.
// Differences from the strict parser (parseCORS):
//   - Origins are optional (not required).
//   - Records which keys were explicitly provided via a CORSFieldMask,
//   - allowing presence-aware merging (e.g., only update MaxAge if set).
//   - Deduplicates merged values.
//   - Applies conflict rules for MaxAge.
//
// PatchCORS uses this looser parser to implement update semantics.
func parseCORSLooseWithMask(chunks []string) (storage.CORSConfig, corsFieldMask, error) {
	var out storage.CORSConfig
	var mask corsFieldMask

	if len(chunks) == 0 {
		return out, mask, nil
	}

	pairs, err := parseSegmentsLoose(chunks)
	if err != nil {
		return out, mask, err
	}

	origins := map[string]struct{}{}
	respHdrs := map[string]struct{}{}
	var maxAgeSeen bool
	var maxAgeVal int

	for _, p := range pairs {
		switch p.key {
		case corsKeyOrigins:
			mask.Origins = true
			for _, o := range splitCSV(p.val) {
				o = strings.TrimSpace(o)
				if o != "" {
					origins[o] = struct{}{}
				}
			}
		case corsKeyResponseHeaders:
			mask.ResponseHeaders = true
			for _, h := range splitCSV(p.val) {
				h = strings.TrimSpace(h)
				if h != "" {
					respHdrs[h] = struct{}{}
				}
			}
		case corsKeyMaxAge:
			mask.MaxAge = true
			// NOTE: value may be "", which is fine; PatchCORS will forbid any presence in --delete-cors.
			if p.val != "" {
				age, err := strconv.Atoi(p.val)
				if err != nil || age < 0 {
					return out, mask, fmt.Errorf("invalid %s=%q (want non-negative int)", corsKeyMaxAge, p.val)
				}
				if !maxAgeSeen {
					maxAgeSeen, maxAgeVal = true, age
				} else if age != maxAgeVal {
					return out, mask, fmt.Errorf("conflicting %s values (%d vs %d); only one %s is allowed",
						corsKeyMaxAge, maxAgeVal, age, corsKeyMaxAge)
				}
			}
		default:
			return out, mask, cli.ErrorWithContext(fmt.Errorf("unknown CORS key %q", p.key)).
				WithExitCode(cli.ExitUsageError).
				WithAvailable(corsKeyOrigins, corsKeyResponseHeaders, corsKeyMaxAge).
				WithSuggestions("Example: --cors='origins=https://example.com;max-age=3600'")
		}
	}

	if len(origins) > 0 {
		out.Origins = make([]string, 0, len(origins))
		for o := range origins {
			out.Origins = append(out.Origins, o)
		}
		sort.Strings(out.Origins)
	}
	if len(respHdrs) > 0 {
		out.ResponseHeaders = make([]string, 0, len(respHdrs))
		for h := range respHdrs {
			out.ResponseHeaders = append(out.ResponseHeaders, h)
		}
		sort.Strings(out.ResponseHeaders)
	}
	if maxAgeSeen {
		out.MaxAge = maxAgeVal
	}
	return out, mask, nil
}

const kvPairSep = "="

type kvPair struct {
	key string
	val string
	// segmentIndex is 1-based, for better error messages
	segmentIndex int
	// Original raw segment (useful for diagnostics)
	raw string
}

type parseOpts struct {
	// When true, "key" (no '=') is accepted and treated like "key=".
	loose bool
	// When true, empty/whitespace-only segments are ignored.
	skipEmpty bool
	// When true, keys are lowercased (or otherwise normalized).
	normalizeKeys bool
	// Separator between segments (typically ';').
	sep string
}

func normalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func splitSegments(raw, sep string) []string {
	if sep == "" {
		sep = ";"
	}
	return strings.Split(raw, sep)
}

// parseKVPairs powers both strict+loose modes.
// - strict: segment must be "k=v" and k cannot be empty.
// - loose: "k" or "k=" or "k=v" are all accepted; never errors on missing '='.
func parseKVPairs(chunks []string, opt parseOpts) ([]kvPair, error) {
	out := make([]kvPair, 0, len(chunks))
	for i, raw := range chunks {
		r := strings.TrimSpace(raw)
		if opt.skipEmpty && r == "" {
			continue
		}

		parts := strings.SplitN(r, kvPairSep, 2)
		switch len(parts) {
		case 1:
			// "key"
			if !opt.loose {
				return nil, fmt.Errorf("segment %d %q must be key=value", i+1, raw)
			}
			k := strings.TrimSpace(parts[0])
			if opt.normalizeKeys {
				k = normalizeKey(k)
			}
			if k == "" {
				return nil, fmt.Errorf("segment %d has empty key", i+1)
			}
			out = append(out, kvPair{
				key:          k,
				val:          "",
				segmentIndex: i + 1,
				raw:          raw,
			})
		case 2:
			// "key=value" or "key="
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if opt.normalizeKeys {
				k = normalizeKey(k)
			}
			if k == "" {
				return nil, fmt.Errorf("segment %d has empty key", i+1)
			}
			out = append(out, kvPair{
				key:          k,
				val:          v,
				segmentIndex: i + 1,
				raw:          raw,
			})
		}
	}
	return out, nil
}

// parseSegmentsStrict parses a list of raw "k=v" segments (already split by ";")
// into normalized kvPair entries. It enforces strict syntax:
//   - Each non-empty segment must be of the form "key=value".
//   - Keys must be non-empty (values may be empty).
//   - Whitespace around keys/values is trimmed.
//
// Returns an error on malformed segments.
func parseSegmentsStrict(chunks []string) ([]kvPair, error) {
	return parseKVPairs(chunks, parseOpts{
		loose:         false,
		skipEmpty:     true,
		normalizeKeys: false, // preserve original case in strict
		sep:           ";",
	})
}

// parseSegmentsLoose tokenizes a list of segments in a tolerant way.
// Differences from strict parsing:
//   - Accepts "key" or "key=" and treats both as an empty value.
//   - Never errors on a missing '=', leaving validation to higher layers.
//   - Trims whitespace around keys/values; key case is preserved.
//
// Intended for update flows (add/remove) where "leniency" is required.
// For create flows, prefer parseSegmentsStrict to enforce "key=value".
func parseSegmentsLoose(chunks []string) ([]kvPair, error) {
	return parseKVPairs(chunks, parseOpts{
		loose:         true,
		skipEmpty:     true,
		normalizeKeys: false,
		sep:           ";",
	})
}

// parseKVMapLoose builds a key-value map from a raw "k=v;..." string.
// It is tolerant: segments without '=' are accepted and treated as "key=".
// Whitespace is trimmed; later duplicates overwrite earlier ones.
func parseKVMapLoose(raw string) map[string]string {
	pairs, _ := parseKVPairs(splitSegments(raw, ";"), parseOpts{
		loose:         true,
		skipEmpty:     true,
		normalizeKeys: false,
		sep:           ";",
	})
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		if p.key != "" {
			m[p.key] = p.val // last-wins
		}
	}
	return m
}

// splitCSV splits a semicolon/comma-delimited string into chunks.
// The semantics are the same as parsing a simple CSV-style field list (no
// quoting/escaping supported).
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
