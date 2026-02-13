package bucket

import (
	"fmt"
	"sort"
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

// PermissionSpec represents a parsed permission entry.
// Each spec binds a role to a (possibly empty) list of users.
type PermissionSpec struct {
	Role  string
	Users []string
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
