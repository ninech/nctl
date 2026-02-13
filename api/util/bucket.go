package util

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/cli"
)

const (
	PermKeyRole  = "role"
	PermKeyUsers = "users"
	permKeyUser  = "user"
)

type stringSet map[string]struct{}

func (s stringSet) Add(v string)      { s[v] = struct{}{} }
func (s stringSet) Del(v string)      { delete(s, v) }
func (s stringSet) Has(v string) bool { _, ok := s[v]; return ok }

// mapKeysSorted returns all keys of the given map in lexicographically sorted order.
// It first collects the map keys into a slice and then sorts them deterministically.
// Useful when iteration over map keys must be stable (e.g. for output, tests, or hashing).
func mapKeysSorted[M ~map[string]V, V any](m M) []string {
	keys := slices.Collect(maps.Keys(m)) // iterator -> []string
	slices.Sort(keys)
	return keys
}

func getOrInitSet(m map[string]stringSet, key string) stringSet {
	if set, ok := m[key]; ok {
		return set
	}
	set := make(stringSet)
	m[key] = set
	return set
}

func setFromSliceTrimmed(in []string) stringSet {
	s := make(stringSet, len(in))
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			s.Add(v)
		}
	}
	return s
}

// PermissionSpec represents a parsed permission entry.
// Each spec binds a role to a (possibly empty) list of users.
type PermissionSpec struct {
	Role  string
	Users []string
}

// General rule I use here:
// parseXxx parses and merges raw flag chunks into structured specs.
// It handles syntax validation, CSV splitting, deduplication, and sorting.
// PatchXxx applies parsed add/remove specs to an existing configuration slice,
// returning a normalized and deterministically ordered result.

// parsePermissionSpecs parses and merges "role=csv(users)" chunks.
// If allowEmptyUsers is true, a role with no users is allowed (used by "remove").
// Returns specs with deterministic ordering (sorted roles & users).
func parsePermissionSpecs(chunks []string, allowEmptyUsers bool) ([]PermissionSpec, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	return parsePermissions(chunks, allowEmptyUsers)
}

func normalizeRole(r string) (string, error) {
	r = strings.ToLower(strings.TrimSpace(r))
	switch r {
	case string(storage.BucketRoleReader), string(storage.BucketRoleWriter):
		return r, nil
	default:
		return "", cli.ErrorWithContext(fmt.Errorf("unknown %s %q", PermKeyRole, r)).
			WithExitCode(cli.ExitUsageError).
			WithAvailable(string(storage.BucketRoleReader), string(storage.BucketRoleWriter)).
			WithSuggestions("Example: --permissions=reader=user1,user2")
	}
}

// PatchPermissions applies additive and subtractive permission specs on top of an
// existing slice of BucketPermission objects. The input slices `add` and `remove`
// are raw flag values (e.g. "reader=user1,user2"), which are parsed and merged
// deterministically. Behavior:
//   - `add`: users are added to the given role (duplicates deduped).
//   - `remove`: users are removed from the given role.
//   - `remove` with no users removes the whole role.
//   - Empty/whitespace users are rejected.
//
// The result is normalized so that roles and user names are sorted and duplicate-free.
//
// Returns an error if role names are invalid, if empty users appear in `add`,
// or if parsing of the specs fails.
func PatchPermissions(base []*storage.BucketPermission, add, remove []string) ([]*storage.BucketPermission, error) {
	if len(add) == 0 && len(remove) == 0 {
		return base, nil
	}

	roleToUsers := map[string]stringSet{}
	for _, p := range base {
		if p == nil {
			continue
		}
		role, err := normalizeRole(string(p.Role))
		if err != nil {
			return nil, err
		}
		set := getOrInitSet(roleToUsers, role)
		for _, ref := range p.BucketUserRefs {
			if ref == nil {
				continue
			}
			name := strings.TrimSpace(ref.Name)
			if name != "" {
				set.Add(name)
			}
		}
	}

	addSpecs, err := parsePermissionSpecs(add, false)
	if err != nil {
		return nil, fmt.Errorf("add permissions: %w", err)
	}
	removeSpecs, err := parsePermissionSpecs(remove, true)
	if err != nil {
		return nil, fmt.Errorf("remove permissions: %w", err)
	}

	for _, spec := range addSpecs {
		role, err := normalizeRole(spec.Role)
		if err != nil {
			return nil, fmt.Errorf("add permissions: %w", err)
		}
		set := getOrInitSet(roleToUsers, role)
		for _, u := range spec.Users {
			u = strings.TrimSpace(u)
			if u == "" {
				return nil, fmt.Errorf("add permissions: empty %s for %s %q", permKeyUser, PermKeyRole, role)
			}
			set.Add(u)
		}
	}

	for _, spec := range removeSpecs {
		role, err := normalizeRole(spec.Role)
		if err != nil {
			return nil, fmt.Errorf("remove permissions: %w", err)
		}
		set, ok := roleToUsers[role]
		if !ok {
			continue
		}
		if len(spec.Users) == 0 {
			delete(roleToUsers, role)
			continue
		}
		for _, u := range spec.Users {
			u = strings.TrimSpace(u)
			if u == "" {
				return nil, fmt.Errorf("remove permissions: empty %s for %s %q", permKeyUser, PermKeyRole, role)
			}
			set.Del(u)
		}
	}

	out := roleUsersToPermissions(roleToUsers)
	return out, nil
}

// parsePermissions converts raw CLI flag chunks into PermissionSpec values.
// It handles syntax validation, CSV splitting, deduplication, and the
// allowEmptyUsers rule. Internally it uses StringSet to manage users per role.
func parsePermissions(chunks []string, allowEmptyUsers bool) ([]PermissionSpec, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	pairs, err := parseSegmentsStrict(chunks)
	if err != nil {
		return nil, err
	}

	byRole := map[string]stringSet{}

	for _, p := range pairs {
		role := strings.TrimSpace(p.key)
		if role == "" {
			return nil, fmt.Errorf("segment %d has empty %s", p.segmentIndex, PermKeyRole)
		}

		csv := strings.TrimSpace(p.val)
		if csv == "" {
			if !allowEmptyUsers {
				return nil, fmt.Errorf("%s %q has no %s", PermKeyRole, role, PermKeyUsers)
			}
			if _, ok := byRole[role]; !ok {
				byRole[role] = stringSet{}
			}
			continue
		}

		users := splitCSV(csv)
		set := byRole[role]
		if set == nil {
			set = stringSet{}
			byRole[role] = set
		}

		added := 0
		for _, u := range users {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			if !set.Has(u) {
				set.Add(u)
				added++
			}
		}

		// If after trimming all were empty and empties are not allowed, complain.
		if added == 0 && !allowEmptyUsers {
			return nil, fmt.Errorf("%s %q has no %s", PermKeyRole, role, PermKeyUsers)
		}
	}

	// Deterministic output (sorted roles & users).
	roles := mapKeysSorted(byRole)
	out := make([]PermissionSpec, 0, len(roles))
	for _, r := range roles {
		set := byRole[r]
		if len(set) == 0 {
			// Empty users on purpose (only when allowEmptyUsers was true in practice)
			out = append(out, PermissionSpec{Role: r, Users: nil})
			continue
		}
		names := make([]string, 0, len(set))
		for name := range set {
			names = append(names, name)
		}
		sort.Strings(names)
		out = append(out, PermissionSpec{Role: r, Users: names})
	}
	return out, nil
}

// roleUsersToPermissions turns role->users set into a deterministically sorted slice.
func roleUsersToPermissions(m map[string]stringSet) []*storage.BucketPermission {
	roles := mapKeysSorted(m)
	out := make([]*storage.BucketPermission, 0, len(roles))

	for _, r := range roles {
		users := m[r]

		// Collect user names deterministically.
		names := make([]string, 0, len(users))
		for name := range users {
			names = append(names, name)
		}
		sort.Strings(names)

		refs := make([]*meta.LocalReference, 0, len(names))
		for _, n := range names {
			refs = append(refs, &meta.LocalReference{Name: n})
		}

		// We always append the role, even if refs is empty.
		out = append(out, &storage.BucketPermission{
			Role:           storage.BucketRole(r),
			BucketUserRefs: refs,
		})
	}
	return out
}

type LifecycleSpec struct {
	Prefix          string
	ExpireAfterDays *int32
	IsLive          bool
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

func LifecycleFromMap(m map[string]string) (LifecycleSpec, error) {
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

func ToBucketLifecyclePolicy(ls LifecycleSpec) *storage.BucketLifecyclePolicy {
	out := &storage.BucketLifecyclePolicy{
		Prefix: ls.Prefix,
		IsLive: ls.IsLive,
	}
	if ls.ExpireAfterDays != nil {
		out.ExpireAfterDays = *ls.ExpireAfterDays
	}
	return out
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
			spec, err := LifecycleFromMap(m)
			if err != nil {
				return nil, fmt.Errorf("lifecycle-policy #%d parsing error: %w", idx+1, err)
			}
			prefix := strings.TrimSpace(spec.Prefix)
			if prefix == "" {
				return nil, fmt.Errorf("lifecycle-policy #%d: missing required key %q", idx+1, "prefix")
			}
			// last-wins on duplicate prefixes
			byPrefix[prefix] = ToBucketLifecyclePolicy(spec)
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

// CORSFieldMask indicates which keys were explicitly present across chunks.
type CORSFieldMask struct {
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
func parseCORSLooseWithMask(chunks []string) (storage.CORSConfig, CORSFieldMask, error) {
	var out storage.CORSConfig
	var mask CORSFieldMask

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

// PatchCustomHostnames applies add/clear/delete operations to a slice of
// BucketCustomHostname entries. It follows the common "patch" pattern:
//   - If clear is true, the result is an empty list regardless of input.
//   - Entries in add are parsed and replace any existing ones with the same host.
//   - Entries in remove identify hostnames to be removed.
func PatchCustomHostnames(base []string, clear bool, add []string, remove []string) ([]string, error) {
	var set stringSet
	if clear {
		set = make(stringSet)
	} else {
		set = setFromSliceTrimmed(base)
	}

	for _, h := range add {
		if h = strings.TrimSpace(h); h != "" {
			set.Add(h)
		}
	}
	for _, h := range remove {
		if h = strings.TrimSpace(h); h != "" {
			set.Del(h)
		}
	}

	return mapKeysSorted(set), nil
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
