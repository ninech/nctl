package bucket

import (
	"strings"
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
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
			t.Parallel()
			is := require.New(t)

			got, err := parseSegmentsStrict(tt.chunks)
			if tt.wantErr != "" {
				is.Error(err)
				is.Contains(err.Error(), tt.wantErr)
				return
			}
			is.NoError(err)
			is.Equal(tt.wantPairs, got)
		})
	}
}

func TestParseCORSLooseWithMask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		chunks    []string
		want      storage.CORSConfig
		wantMask  corsFieldMask
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
			wantMask: corsFieldMask{Origins: true, ResponseHeaders: true, MaxAge: true},
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
			wantMask: corsFieldMask{Origins: true, ResponseHeaders: true},
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
			wantMask: corsFieldMask{Origins: true, ResponseHeaders: true},
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
			wantMask: corsFieldMask{Origins: true, MaxAge: true},
		},
		{
			name: "ok: only max-age provided",
			chunks: []string{
				"max-age=1800",
			},
			want:     storage.CORSConfig{MaxAge: 1800},
			wantMask: corsFieldMask{MaxAge: true},
			nilFields: struct {
				origins, responseHeaders bool
			}{origins: true, responseHeaders: true},
		},
		{
			name:     "ok: no chunks means zero values; CRD will inject defaults later",
			chunks:   nil,
			want:     storage.CORSConfig{MaxAge: 0},
			wantMask: corsFieldMask{},
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
			wantErr: "unknown CORS key",
		},
		{
			name: "error: bad segment format (loose tokenizer yields unknown key here)",
			chunks: []string{
				"origins:https://example.com",
			},
			wantErr: "unknown CORS key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			got, gotMask, err := parseCORSLooseWithMask(tt.chunks)

			if tt.wantErr != "" {
				is.Error(err)
				is.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErr))
				return
			}
			is.NoError(err)
			is.Equal(tt.wantMask, gotMask)

			if tt.nilFields.origins {
				is.Nil(got.Origins, "Origins should be nil")
			} else {
				is.Equal(tt.want.Origins, got.Origins)
			}
			if tt.nilFields.responseHeaders {
				is.Nil(got.ResponseHeaders, "ResponseHeaders should be nil")
			} else {
				is.Equal(tt.want.ResponseHeaders, got.ResponseHeaders)
			}

			is.Equal(tt.want.MaxAge, got.MaxAge)
		})
	}
}

func TestParseKVPairsStrict(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		is := require.New(t)

		kv, err := parseSegmentsStrict([]string{"a=1", " b = 2 ", "  "})
		is.NoError(err)
		is.Equal([]kvPair{
			{key: "a", val: "1", segmentIndex: 1, raw: "a=1"},
			{key: "b", val: "2", segmentIndex: 2, raw: " b = 2 "},
		}, kv)
	})
	t.Run("error on missing equals", func(t *testing.T) {
		t.Parallel()
		is := require.New(t)

		_, err := parseSegmentsStrict([]string{"a"})
		is.Error(err)
		is.Contains(err.Error(), `must be key=value`)
	})
	t.Run("error on empty key", func(t *testing.T) {
		t.Parallel()
		is := require.New(t)

		_, err := parseSegmentsStrict([]string{"=1"})
		is.Error(err)
		is.Contains(err.Error(), `empty key`)
	})
}

func TestParseKVPairsLoose(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	kv, err := parseSegmentsLoose([]string{"a", "b=", "c=3", "  "})
	is.NoError(err)
	is.Equal([]kvPair{
		{key: "a", val: "", segmentIndex: 1, raw: "a"},
		{key: "b", val: "", segmentIndex: 2, raw: "b="},
		{key: "c", val: "3", segmentIndex: 3, raw: "c=3"},
	}, kv)
}

func TestParseKVMapLoose(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	m := parseKVMapLoose("prefix=logs/;is-live=true;note")
	is.Equal("logs/", m["prefix"])
	is.Equal("true", m["is-live"])
	is.Equal("", m["note"])
}
