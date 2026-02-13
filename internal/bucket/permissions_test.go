package bucket

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
			t.Parallel()
			is := require.New(t)

			got, err := parsePermissions(tt.chunks, tt.allowEmptyUsers)
			if tt.wantErr != "" {
				is.Error(err)
				is.Contains(err.Error(), tt.wantErr)
				return
			}
			is.NoError(err)
			is.Equal(tt.want, got)
		})
	}
}
