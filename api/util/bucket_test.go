package util

import (
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
