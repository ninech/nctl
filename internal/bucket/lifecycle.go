// Package bucket provides utilities for managing buckets.
package bucket

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	storage "github.com/ninech/apis/storage/v1alpha1"
)

type LifecycleSpec struct {
	Prefix          string
	ExpireAfterDays *int32
	IsLive          bool
}

// PatchLifecyclePolicies applies a sequence of operations to a set of
// bucket lifecycle policies. Operations are applied in the following order:
//   - Clear: optionally start from an empty set, discarding all existing policies.
//   - Add: merge new or updated policies into the working set. Policies are
//     keyed by prefix; providing the same prefix again replaces the previous entry.
//   - Delete: remove policies by prefix from the working set.
//
// The result is a deterministic, prefix-sorted slice.
func PatchLifecyclePolicies(
	base []*storage.BucketLifecyclePolicy,
	clear bool,
	addChunks []string,
	deleteChunks []string,
) ([]*storage.BucketLifecyclePolicy, error) {
	if !clear && len(addChunks) == 0 && len(deleteChunks) == 0 {
		return base, nil
	}

	byPrefix := map[string]*storage.BucketLifecyclePolicy{}

	if clear {
		// Clear means "start from empty", even if base had entries.
	} else {
		// Seed from base. We only index by trimmed, non-empty prefixes.
		// Note: shallow copy is sufficient because we don't mutate the
		// existing policy objects; we only replace map entries or drop them.
		for _, p := range base {
			if p == nil {
				continue
			}
			px := strings.TrimSpace(p.Prefix)
			if px == "" {
				continue
			}
			byPrefix[px] = p
		}
	}

	if len(addChunks) > 0 {
		for idx, raw := range addChunks {
			m := parseKVMapLoose(raw)
			spec, err := lifecycleFromMap(m)
			if err != nil {
				return nil, fmt.Errorf("lifecycle-policy #%d parsing error: %w", idx+1, err)
			}
			prefix := strings.TrimSpace(spec.Prefix)
			if prefix == "" {
				return nil, fmt.Errorf("lifecycle-policy #%d: missing required key %q", idx+1, "prefix")
			}
			// last-wins on duplicate prefixes
			byPrefix[prefix] = bucketLifecyclePolicy(spec)
		}
	}

	if len(deleteChunks) > 0 {
		prefixes, err := parseDeleteLifecyclePrefixes(deleteChunks)
		if err != nil {
			return nil, err
		}
		for px := range prefixes {
			delete(byPrefix, px)
		}
	}

	prefixes := make([]string, 0, len(byPrefix))
	for k := range byPrefix {
		prefixes = append(prefixes, k)
	}
	sort.Strings(prefixes)

	out := make([]*storage.BucketLifecyclePolicy, 0, len(prefixes))
	for _, px := range prefixes {
		out = append(out, byPrefix[px])
	}

	return out, nil
}

// bucketLifecyclePolicy converts a LifecycleSpec into a storage.BucketLifecyclePolicy.
func bucketLifecyclePolicy(ls LifecycleSpec) *storage.BucketLifecyclePolicy {
	out := &storage.BucketLifecyclePolicy{
		Prefix: ls.Prefix,
		IsLive: ls.IsLive,
	}
	if ls.ExpireAfterDays != nil {
		out.ExpireAfterDays = *ls.ExpireAfterDays
	}
	return out
}

// parseDeleteLifecyclePrefixes extracts required 'prefix=' from each chunk.
// Example inputs (repeatable):
//
//	"prefix=logs/"
//	"prefix=tmp/;is-live=true"    // allowed; we only care about prefix here
//
// Each delete chunk must include prefix=<...>. Other keys are ignored (but allowed).
func parseDeleteLifecyclePrefixes(chunks []string) (stringSet, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	out := make(stringSet, len(chunks))
	for i, raw := range chunks {
		m := parseKVMapLoose(raw)
		prefix := strings.TrimSpace(m["prefix"])
		if prefix == "" {
			return nil, fmt.Errorf("--delete-lifecycle-policy #%d: missing required key %q", i+1, "prefix")
		}
		out.Add(prefix)
	}
	return out, nil
}

// lifecycleFromMap converts a raw key-value map into a LifecycleSpec,
// validating required fields and parsing numeric/boolean values.
func lifecycleFromMap(m map[string]string) (LifecycleSpec, error) {
	ls := LifecycleSpec{
		Prefix: strings.TrimSpace(m["prefix"]),
	}
	if v := strings.TrimSpace(m["is-live"]); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return ls, fmt.Errorf("invalid is-live=%q", v)
		}
		ls.IsLive = b
	}

	// Note: v1 creation is no longer supported.
	v2 := strings.TrimSpace(m["expire-after-days"])
	if v2 == "" {
		return ls, fmt.Errorf("missing expiry (expire-after OR expire-after-days)")
	}

	days, err := strconv.Atoi(v2)
	if err != nil || days < 0 {
		return ls, fmt.Errorf("invalid expire-after-days=%q (want non-negative int)", v2)
	}
	dd := int32(days)
	ls.ExpireAfterDays = &dd

	return ls, nil
}
