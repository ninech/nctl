package util

import (
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestParseSegments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		chunks    []string
		wantPairs []kvPair
		wantErr   string
	}{
		{
			name:      "simple input",
			chunks:    []string{"key=value"},
			wantPairs: []kvPair{{key: "key", val: "value", segmentIndex: 1, raw: "key=value"}},
		},
		{
			name:      "empty input",
			chunks:    nil,
			wantPairs: []kvPair{},
		},
		{
			name:      "skips empty segments",
			chunks:    []string{"   ", "", "key=value", "  "}, // removed "  ;  "
			wantPairs: []kvPair{{key: "key", val: "value", segmentIndex: 3, raw: "key=value"}},
		},
		{
			name:    "errors on key only",
			chunks:  []string{"keyonly"},
			wantErr: `segment 1 "keyonly" must be key=value`,
		},
		{
			// TODO: not sure I should allow this:
			name:      "empty value allowed",
			chunks:    []string{"keyonly="},
			wantPairs: []kvPair{{key: "keyonly", val: "", segmentIndex: 1, raw: "keyonly="}},
		},
		{
			name:    "errors on empty key",
			chunks:  []string{" =value "},
			wantErr: "segment 1 has empty key",
		},
		{
			name:      "trims spaces around key and value",
			chunks:    []string{"  k  =  v  "},
			wantPairs: []kvPair{{key: "k", val: "v", segmentIndex: 1, raw: "  k  =  v  "}},
		},
		{
			name:      "multiple valid pairs",
			chunks:    []string{"a=1", "b=2"},
			wantPairs: []kvPair{{key: "a", val: "1", segmentIndex: 1, raw: "a=1"}, {key: "b", val: "2", segmentIndex: 2, raw: "b=2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSegmentsStrict(tt.chunks)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPairs, got)
		})
	}
}

func TestParsePermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		chunks          []string
		allowEmptyUsers bool
		want            []PermissionSpec
		wantErr         string
	}{
		{
			name: "ok: single role with multiple users",
			chunks: []string{
				"reader=test-user-1,guest-user-2",
			},
			allowEmptyUsers: false,
			want: []PermissionSpec{
				{Role: "reader", Users: []string{"guest-user-2", "test-user-1"}},
			},
		},
		{
			name: "ok: multiple roles, merged & deduped users",
			chunks: []string{
				"reader=test-user-1,test-user-1",
				"writer=super-user-1",
				"reader=guest-user-2",
			},
			allowEmptyUsers: false,
			want: []PermissionSpec{
				{Role: "reader", Users: []string{"guest-user-2", "test-user-1"}},
				{Role: "writer", Users: []string{"super-user-1"}},
			},
		},
		{
			name: "error: missing users when not allowed",
			chunks: []string{
				"reader=",
			},
			allowEmptyUsers: false,
			wantErr:         "no users",
		},
		{
			name: "ok: explicit role-only is allowed for deletes",
			chunks: []string{
				"writer=",
			},
			allowEmptyUsers: true,
			// Users should be omitted (nil) to indicate role-only spec
			want: []PermissionSpec{
				{Role: "writer", Users: nil},
			},
		},
		{
			name: "error: malformed segment",
			chunks: []string{
				"reader test-user-1",
			},
			allowEmptyUsers: false,
			wantErr:         "must be key=value",
		},
		{
			name: "ok: trims empties in CSV; still errors if nothing left when not allowed",
			chunks: []string{
				"reader= ,  ,",
			},
			allowEmptyUsers: false,
			wantErr:         "no users",
		},
		{
			name: "ok: trims empties in CSV; allowed when allowEmptyUsers=true",
			chunks: []string{
				"reader= ,  ,",
			},
			allowEmptyUsers: true,
			want: []PermissionSpec{
				{Role: "reader", Users: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePermissions(tt.chunks, tt.allowEmptyUsers)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// helper: build a permission with role and a series of user names.
func perm(role storage.BucketRole, names ...*string) *storage.BucketPermission {
	refs := make([]*meta.LocalReference, len(names))
	for i, n := range names {
		if n == nil {
			refs[i] = nil
			continue
		}
		refs[i] = &meta.LocalReference{Name: *n}
	}
	return &storage.BucketPermission{
		Role:           role,
		BucketUserRefs: refs,
	}
}

func TestEqualPermissions(t *testing.T) {
	tests := []struct {
		name string
		a    []*storage.BucketPermission
		b    []*storage.BucketPermission
		want bool
	}{
		{
			name: "both nil slices",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "empty slices",
			a:    []*storage.BucketPermission{},
			b:    []*storage.BucketPermission{},
			want: true,
		},
		{
			name: "different slice lengths",
			a:    []*storage.BucketPermission{perm("reader", ptr.To("alice"))},
			b:    []*storage.BucketPermission{},
			want: false,
		},
		{
			name: "different roles",
			a: []*storage.BucketPermission{
				perm("reader", ptr.To("alice")),
				perm("writer", ptr.To("bob")),
			},
			b: []*storage.BucketPermission{
				perm("reader", ptr.To("alice")),
				perm("owner", ptr.To("bob")),
			},
			want: false,
		},
		{
			name: "different userRefs length",
			a:    []*storage.BucketPermission{perm("reader", ptr.To("alice"))},
			b:    []*storage.BucketPermission{perm("reader", ptr.To("alice"), ptr.To("bob"))},
			want: false,
		},
		{
			name: "nil user refs at same positions",
			a:    []*storage.BucketPermission{perm("reader", nil, ptr.To("alice"))},
			b:    []*storage.BucketPermission{perm("reader", nil, ptr.To("alice"))},
			want: true,
		},
		{
			name: "nil vs non-nil user ref",
			a:    []*storage.BucketPermission{perm("reader", nil, ptr.To("alice"))},
			b:    []*storage.BucketPermission{perm("reader", ptr.To("ghost"), ptr.To("alice"))},
			want: false,
		},
		{
			name: "same roles & users",
			a: []*storage.BucketPermission{
				perm("owner", ptr.To("alice"), ptr.To("bob")),
				perm("reader", ptr.To("carol")),
			},
			b: []*storage.BucketPermission{
				perm("owner", ptr.To("alice"), ptr.To("bob")),
				perm("reader", ptr.To("carol")),
			},
			want: true,
		},
		{
			name: "user name mismatch",
			a:    []*storage.BucketPermission{perm("reader", ptr.To("bob"))},
			b:    []*storage.BucketPermission{perm("reader", ptr.To("b0b"))},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalPermissions(tt.a, tt.b)
			assert.Equal(t, tt.want, got)

			// symmetry check
			gotRev := equalPermissions(tt.b, tt.a)
			assert.Equal(t, tt.want, gotRev)
		})
	}
}

func TestEqualLifecyclePolicy(t *testing.T) {
	tests := []struct {
		name string
		a, b *storage.BucketLifecyclePolicy
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "nil vs non-nil",
			a:    nil,
			b: &storage.BucketLifecyclePolicy{
				Prefix:          "p/",
				ExpireAfterDays: int32(0),
				IsLive:          true,
			},
			want: false,
		},
		{
			name: "identical values",
			a: &storage.BucketLifecyclePolicy{
				Prefix:          "logs/",
				ExpireAfterDays: int32(7),
				IsLive:          true,
			},
			b: &storage.BucketLifecyclePolicy{
				Prefix:          "logs/",
				ExpireAfterDays: int32(7),
				IsLive:          true,
			},
			want: true,
		},
		{
			name: "different prefix",
			a: &storage.BucketLifecyclePolicy{
				Prefix:          "logs/",
				ExpireAfterDays: int32(7),
				IsLive:          true,
			},
			b: &storage.BucketLifecyclePolicy{
				Prefix:          "metrics/",
				ExpireAfterDays: int32(7),
				IsLive:          true,
			},
			want: false,
		},
		{
			name: "different ExpireAfterDays",
			a: &storage.BucketLifecyclePolicy{
				Prefix:          "logs/",
				ExpireAfterDays: int32(30),
				IsLive:          true,
			},
			b: &storage.BucketLifecyclePolicy{
				Prefix:          "logs/",
				ExpireAfterDays: int32(31),
				IsLive:          true,
			},
			want: false,
		},
		{
			name: "different IsLive",
			a: &storage.BucketLifecyclePolicy{
				Prefix:          "logs/",
				ExpireAfterDays: int32(0),
				IsLive:          true,
			},
			b: &storage.BucketLifecyclePolicy{
				Prefix:          "logs/",
				ExpireAfterDays: int32(0),
				IsLive:          false,
			},
			want: false,
		},
		{
			name: "both zero-values (equal)",
			a:    &storage.BucketLifecyclePolicy{
				// Prefix "", ExpireAfterDays 0, IsLive false (zero values)
			},
			b: &storage.BucketLifecyclePolicy{
				// Prefix "", ExpireAfterDays 0, IsLive false (zero values)
			},
			want: true,
		},
		{
			name: "same pointer (trivially equal)",
			a: func() *storage.BucketLifecyclePolicy {
				p := &storage.BucketLifecyclePolicy{
					Prefix:          "same/",
					ExpireAfterDays: int32(3),
					IsLive:          false,
				}
				return p
			}(),
			b:    nil, // will be set to a in the test body
			want: true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ensure the "same pointer" case actually uses the same address
			if tt.name == "same pointer (trivially equal)" {
				tests[i].b = tests[i].a
				tt.b = tt.a
			}

			got := equalLifecyclePolicy(tt.a, tt.b)
			assert.Equal(t, tt.want, got, "equalLifecyclePolicy(%v, %v)", tt.a, tt.b)

			// symmetry check
			gotRev := equalLifecyclePolicy(tt.b, tt.a)
			assert.Equal(t, tt.want, gotRev, "symmetry check failed")
		})
	}
}

func TestEqualCORS(t *testing.T) {
	tests := []struct {
		name string
		a, b *storage.CORSConfig
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "nil vs non-nil",
			a:    nil,
			b: &storage.CORSConfig{
				Origins:         []string{"https://example.com"},
				ResponseHeaders: []string{"X-Test"},
				MaxAge:          3600,
			},
			want: false,
		},
		{
			name: "identical values",
			a: &storage.CORSConfig{
				Origins:         []string{"https://a.com", "https://b.com"},
				ResponseHeaders: []string{"X-A", "X-B"},
				MaxAge:          600,
			},
			b: &storage.CORSConfig{
				Origins:         []string{"https://a.com", "https://b.com"},
				ResponseHeaders: []string{"X-A", "X-B"},
				MaxAge:          600,
			},
			want: true,
		},
		{
			name: "different MaxAge",
			a: &storage.CORSConfig{
				Origins:         []string{"*"},
				ResponseHeaders: []string{"X-Test"},
				MaxAge:          100,
			},
			b: &storage.CORSConfig{
				Origins:         []string{"*"},
				ResponseHeaders: []string{"X-Test"},
				MaxAge:          200,
			},
			want: false,
		},
		{
			name: "different Origins length",
			a: &storage.CORSConfig{
				Origins:         []string{"https://a.com"},
				ResponseHeaders: []string{},
				MaxAge:          0,
			},
			b: &storage.CORSConfig{
				Origins:         []string{"https://a.com", "https://b.com"},
				ResponseHeaders: []string{},
				MaxAge:          0,
			},
			want: false,
		},
		{
			name: "different Origins values",
			a: &storage.CORSConfig{
				Origins:         []string{"https://a.com"},
				ResponseHeaders: []string{},
				MaxAge:          0,
			},
			b: &storage.CORSConfig{
				Origins:         []string{"https://b.com"},
				ResponseHeaders: []string{},
				MaxAge:          0,
			},
			want: false,
		},
		{
			name: "different ResponseHeaders length",
			a: &storage.CORSConfig{
				Origins:         []string{"*"},
				ResponseHeaders: []string{"X-A"},
				MaxAge:          0,
			},
			b: &storage.CORSConfig{
				Origins:         []string{"*"},
				ResponseHeaders: []string{"X-A", "X-B"},
				MaxAge:          0,
			},
			want: false,
		},
		{
			name: "different ResponseHeaders values",
			a: &storage.CORSConfig{
				Origins:         []string{"*"},
				ResponseHeaders: []string{"X-A"},
				MaxAge:          0,
			},
			b: &storage.CORSConfig{
				Origins:         []string{"*"},
				ResponseHeaders: []string{"X-B"},
				MaxAge:          0,
			},
			want: false,
		},
		{
			name: "both zero-values (equal)",
			a:    &storage.CORSConfig{},
			b:    &storage.CORSConfig{},
			want: true,
		},
		{
			name: "same pointer (trivially equal)",
			a: &storage.CORSConfig{
				Origins:         []string{"https://same.com"},
				ResponseHeaders: []string{"X-Same"},
				MaxAge:          123,
			},
			b:    nil, // will be set in the test body
			want: true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// force same pointer case
			if tt.name == "same pointer (trivially equal)" {
				tests[i].b = tests[i].a
				tt.b = tt.a
			}

			got := equalCORS(tt.a, tt.b)
			assert.Equal(t, tt.want, got)

			// symmetry check
			gotRev := equalCORS(tt.b, tt.a)
			assert.Equal(t, tt.want, gotRev)
		})
	}
}

func TestParseCORSLooseWithMask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		chunks    []string
		want      storage.CORSConfig
		wantMask  CORSFieldMask
		wantErr   string
		nilFields struct {
			origins         bool
			responseHeaders bool
		}
	}{
		{
			name: "ok: single flag with all keys",
			chunks: []string{
				"origins=https://example.com, https://app.example.com ",
				"response-headers= X-My-Header ,ETag ",
				"max-age=3600",
			},
			want: storage.CORSConfig{
				Origins:         []string{"https://app.example.com", "https://example.com"},
				ResponseHeaders: []string{"ETag", "X-My-Header"},
				MaxAge:          3600,
			},
			wantMask: CORSFieldMask{Origins: true, ResponseHeaders: true, MaxAge: true},
		},
		{
			name: "ok: multiple flags merged & deduped; max-age not provided -> stays zero (unset here, CRD will default later)",
			chunks: []string{
				"origins=https://example.com,https://example.com",
				"origins=https://app.example.com",
				"response-headers=ETag",
				"response-headers=X-My-Header",
			},
			want: storage.CORSConfig{
				Origins:         []string{"https://app.example.com", "https://example.com"},
				ResponseHeaders: []string{"ETag", "X-My-Header"},
				MaxAge:          0, // parser leaves unset
			},
			wantMask: CORSFieldMask{Origins: true, ResponseHeaders: true},
		},
		{
			name: "ok: empty response-headers allowed (treated as none), max-age not provided",
			chunks: []string{
				"origins=https://example.com",
				"response-headers=",
			},
			want: storage.CORSConfig{
				Origins: []string{"https://example.com"},
				MaxAge:  0,
			},
			wantMask: CORSFieldMask{Origins: true, ResponseHeaders: true},
			nilFields: struct {
				origins, responseHeaders bool
			}{responseHeaders: true},
		},
		{
			name: "ok: empty max-age value allowed; mask flips but value stays zero",
			chunks: []string{
				"origins=https://example.com",
				"max-age=",
			},
			want: storage.CORSConfig{
				Origins: []string{"https://example.com"},
				MaxAge:  0,
			},
			wantMask: CORSFieldMask{Origins: true, MaxAge: true},
		},
		{
			name: "ok: only max-age provided",
			chunks: []string{
				"max-age=1800",
			},
			want:     storage.CORSConfig{MaxAge: 1800},
			wantMask: CORSFieldMask{MaxAge: true},
			nilFields: struct {
				origins, responseHeaders bool
			}{origins: true, responseHeaders: true},
		},
		{
			name:     "ok: no chunks means zero values; CRD will inject defaults later",
			chunks:   nil,
			want:     storage.CORSConfig{MaxAge: 0},
			wantMask: CORSFieldMask{},
			nilFields: struct {
				origins, responseHeaders bool
			}{origins: true, responseHeaders: true},
		},
		{
			name: "error: conflicting max-age values",
			chunks: []string{
				"origins=https://example.com",
				"max-age=3600",
				"max-age=1800",
			},
			wantErr: "conflicting max-age values",
		},
		{
			name: "error: invalid max-age (non-int)",
			chunks: []string{
				"origins=https://example.com",
				"max-age=ten",
			},
			wantErr: "invalid max-age",
		},
		{
			name: "error: unknown key",
			chunks: []string{
				"origins=https://example.com",
				"method=GET",
			},
			wantErr: "unknown key",
		},
		{
			name: "error: bad segment format (loose tokenizer yields unknown key here)",
			chunks: []string{
				"origins:https://example.com",
			},
			wantErr: "unknown key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotMask, err := parseCORSLooseWithMask(tt.chunks)

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantMask, gotMask)

			if tt.nilFields.origins {
				assert.Nil(t, got.Origins, "Origins should be nil")
			} else {
				assert.Equal(t, tt.want.Origins, got.Origins)
			}
			if tt.nilFields.responseHeaders {
				assert.Nil(t, got.ResponseHeaders, "ResponseHeaders should be nil")
			} else {
				assert.Equal(t, tt.want.ResponseHeaders, got.ResponseHeaders)
			}

			assert.Equal(t, tt.want.MaxAge, got.MaxAge)
		})
	}
}

func TestParseKVPairsStrict(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		kv, err := parseSegmentsStrict([]string{"a=1", " b = 2 ", "  "})
		require.NoError(t, err)
		require.Equal(t, []kvPair{
			{key: "a", val: "1", segmentIndex: 1, raw: "a=1"},
			{key: "b", val: "2", segmentIndex: 2, raw: " b = 2 "},
		}, kv)
	})
	t.Run("error on missing equals", func(t *testing.T) {
		_, err := parseSegmentsStrict([]string{"a"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `must be key=value`)
	})
	t.Run("error on empty key", func(t *testing.T) {
		_, err := parseSegmentsStrict([]string{"=1"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `empty key`)
	})
}

func TestParseKVPairsLoose(t *testing.T) {
	kv, err := parseSegmentsLoose([]string{"a", "b=", "c=3", "  "})
	require.NoError(t, err)
	require.Equal(t, []kvPair{
		{key: "a", val: "", segmentIndex: 1, raw: "a"},
		{key: "b", val: "", segmentIndex: 2, raw: "b="},
		{key: "c", val: "3", segmentIndex: 3, raw: "c=3"},
	}, kv)
}

func TestParseKVMapLoose(t *testing.T) {
	m := parseKVMapLoose("prefix=logs/;is-live=true;note")
	require.Equal(t, "logs/", m["prefix"])
	require.Equal(t, "true", m["is-live"])
	require.Equal(t, "", m["note"])
}
