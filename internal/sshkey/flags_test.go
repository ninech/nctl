package sshkey

import (
	"bytes"
	"io"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"

	"github.com/ninech/nctl/internal/format"
)

// TestFlags parses real arguments through Kong so that the names the embedded
// structs register their flags under, the prefix handling and the messages
// which spell those names out are exercised together.
func TestFlags(t *testing.T) {
	t.Parallel()

	keyFile := openKeyFile(t, "id_ed25519.pub", testPublicKeyB+"\n")

	tests := map[string]struct {
		args           []string
		want           []string
		wantDeprecated []string
		wantWarn       string
		wantErr        string
	}{
		"none": {},
		"inline and file": {
			args: []string{`--rescue-ssh-keys=` + testPublicKeyA, `--rescue-ssh-keys-from-files=` + keyFile.Name()},
			want: []string{testPublicKeyA, testPublicKeyB},
		},
		// sep:"none" keeps the options of an authorized_keys line intact.
		"inline key with options": {
			args: []string{`--rescue-ssh-keys=restrict,pty ` + testPublicKeyA},
			want: []string{`restrict,pty ` + testPublicKeyA},
		},
		"invalid inline key names the flag": {
			args:    []string{`--rescue-ssh-keys=not a key`},
			wantErr: "error reading --rescue-ssh-keys: invalid SSH public key on line 1",
		},
		// the deprecated flags keep splitting on commas.
		"deprecated flags": {
			args:           []string{`--rescue-public-keys=` + testPublicKeyA + `,` + testPublicKeyB},
			wantDeprecated: []string{testPublicKeyA, testPublicKeyB},
			wantWarn:       "--rescue-public-keys and --rescue-public-keys-from-files are deprecated, use --rescue-ssh-keys and --rescue-ssh-keys-from-files instead",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			var cmd struct {
				SSHKeysFlags        `prefix:"rescue-" set:"ssh_keys_purpose=for the tests"`
				DeprecatedKeysFlags `prefix:"rescue-public-"`
			}
			_, err := kong.Must(&cmd, kong.BindTo(io.Discard, (*io.Writer)(nil))).Parse(tt.args)
			is.NoError(err)

			out := &bytes.Buffer{}
			w := format.NewWriter(out)

			keys, err := cmd.SSHKeysFlags.Keys(&w, "rescue-")
			if tt.wantErr != "" {
				is.ErrorContains(err, tt.wantErr)
				return
			}
			is.NoError(err)
			is.Equal(tt.want, keys)

			deprecated, err := cmd.DeprecatedKeysFlags.Keys(&w, "rescue-public-", "rescue-")
			is.NoError(err)
			is.Equal(tt.wantDeprecated, deprecated)

			if tt.wantWarn == "" {
				is.Empty(out.String())
			} else {
				is.Contains(out.String(), tt.wantWarn)
			}
		})
	}
}
