package format

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteBase64Decoded(t *testing.T) {
	t.Parallel()

	const pem = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"

	tests := []struct {
		name      string
		input     string
		want      string
		expectErr bool
	}{
		{
			name:  "decodes and appends newline",
			input: base64.StdEncoding.EncodeToString([]byte(pem)),
			want:  pem + "\n",
		},
		{
			name:  "trims whitespace around encoded and decoded content",
			input: "  \n" + base64.StdEncoding.EncodeToString([]byte("\n"+pem+"\n\n")) + "\n ",
			want:  pem + "\n",
		},
		{
			name:  "empty input writes nothing",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only input writes nothing",
			input: " \n\t",
			want:  "",
		},
		{
			name:      "invalid base64",
			input:     "not base64!",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			out := &bytes.Buffer{}
			err := WriteBase64Decoded(out, tc.input)
			if tc.expectErr {
				is.Error(err)
				is.Empty(out.String())
				return
			}
			is.NoError(err)
			is.Equal(tc.want, out.String())
		})
	}
}
