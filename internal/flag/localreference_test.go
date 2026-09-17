package flag

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalReference_UnmarshalText(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		arg     string
		want    string
		wantErr bool
	}{
		"name":                   {arg: "user1", want: "user1"},
		"surrounding whitespace": {arg: " user1\n", want: "user1"},
		"empty":                  {arg: "", wantErr: true},
		"only whitespace":        {arg: "  ", wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			r := &LocalReference{}
			err := r.UnmarshalText([]byte(tt.arg))
			if tt.wantErr {
				is.Error(err)
				return
			}

			is.NoError(err)
			is.Equal(tt.want, r.Name)
		})
	}
}
