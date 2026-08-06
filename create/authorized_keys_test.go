package create

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Valid ed25519 public keys used across the tests of this package.
const (
	testPublicKeyA = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJQQywLL6rNaZTvaomlhlHVvY36Tq7j1yuxJzBHark/V a@example.com`
	testPublicKeyB = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC3bhEbFGMeJwiB7r2GTr/WLWlxrTG9CxzOTf4fM226g b@example.com`
)

func TestParseAuthorizedKeys(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in      string
		want    []string
		wantErr string
	}{
		"empty":            {in: ""},
		"only comments":    {in: "# nothing to see here\n\n"},
		"single key":       {in: testPublicKeyA, want: []string{testPublicKeyA}},
		"trailing newline": {in: testPublicKeyA + "\n", want: []string{testPublicKeyA}},
		"surrounding whitespace": {
			in: "  " + testPublicKeyA + "  \n", want: []string{testPublicKeyA},
		},
		"multiple keys": {
			in:   testPublicKeyA + "\n" + testPublicKeyB + "\n",
			want: []string{testPublicKeyA, testPublicKeyB},
		},
		"blank and comment lines in between": {
			in:   "# my keys\n" + testPublicKeyA + "\n\n  # another one\n" + testPublicKeyB + "\n",
			want: []string{testPublicKeyA, testPublicKeyB},
		},
		"key without comment": {
			in:   strings.TrimSuffix(testPublicKeyA, " a@example.com"),
			want: []string{strings.TrimSuffix(testPublicKeyA, " a@example.com")},
		},
		"key with options": {
			in:   `no-agent-forwarding ` + testPublicKeyA,
			want: []string{`no-agent-forwarding ` + testPublicKeyA},
		},
		"garbage": {
			in: "not a key\n", wantErr: "invalid SSH public key on line 1",
		},
		"private key": {
			in:      "-----BEGIN OPENSSH PRIVATE KEY-----\n",
			wantErr: "invalid SSH public key on line 1",
		},
		"reports the offending line": {
			in:      "# my keys\n" + testPublicKeyA + "\n\nnot a key\n",
			wantErr: "invalid SSH public key on line 4",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			keys, err := ParseAuthorizedKeys(strings.NewReader(tt.in))
			if tt.wantErr != "" {
				is.ErrorContains(err, tt.wantErr)
				is.Nil(keys)
				return
			}

			is.NoError(err)
			is.Equal(tt.want, keys)
		})
	}
}
