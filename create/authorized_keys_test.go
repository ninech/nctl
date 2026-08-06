package create

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/stretchr/testify/require"

	"github.com/ninech/nctl/internal/format"
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

// parseDatabaseSSHKeys parses args into a [DatabaseSSHKeysFlags] the same way
// the real CLI does, so that Kong's mappers and the flag names the struct is
// registered under are exercised.
func parseDatabaseSSHKeys(t *testing.T, args ...string) DatabaseSSHKeysFlags {
	t.Helper()

	var cmd struct {
		DatabaseSSHKeysFlags `set:"ssh_keys_purpose=for the tests"`
	}
	_, err := kong.Must(&cmd, kong.BindTo(io.Discard, (*io.Writer)(nil))).Parse(args)
	require.NoError(t, err)

	return cmd.DatabaseSSHKeysFlags
}

// TestDatabaseSSHKeysFlags asserts that the SSH key flags of the database
// commands complement each other, including the deprecated --ssh-keys-file
// whose keys used to replace the inline ones instead of adding to them.
func TestDatabaseSSHKeysFlags(t *testing.T) {
	t.Parallel()

	var (
		keyFile     = writeKeyFile(t, "id_ed25519.pub", testPublicKeyB+"\n")
		legacyFile  = writeKeyFile(t, "authorized_keys", "# my keys\n"+testPublicKeyA+"\n")
		invalidFile = writeKeyFile(t, "invalid.pub", "not a key\n")
	)

	const deprecationWarning = "--ssh-keys-file is deprecated, use --ssh-keys-from-files instead"

	tests := map[string]struct {
		args     []string
		want     []storage.SSHKey
		wantWarn string
		wantErr  string
	}{
		"none": {args: nil},
		"inline": {
			args: []string{`--ssh-keys=` + testPublicKeyA},
			want: []storage.SSHKey{testPublicKeyA},
		},
		"file": {
			args: []string{`--ssh-keys-from-files=` + keyFile},
			want: []storage.SSHKey{testPublicKeyB},
		},
		"multiple files": {
			args: []string{`--ssh-keys-from-files=` + keyFile, `--ssh-keys-from-files=` + legacyFile},
			want: []storage.SSHKey{testPublicKeyB, testPublicKeyA},
		},
		"deprecated file": {
			args:     []string{`--ssh-keys-file=` + legacyFile},
			want:     []storage.SSHKey{testPublicKeyA},
			wantWarn: deprecationWarning,
		},
		// the deprecated file used to replace the inline keys entirely.
		"all sources": {
			args: []string{
				`--ssh-keys=` + testPublicKeyA,
				`--ssh-keys-from-files=` + keyFile,
				`--ssh-keys-file=` + legacyFile,
			},
			want:     []storage.SSHKey{testPublicKeyA, testPublicKeyB, testPublicKeyA},
			wantWarn: deprecationWarning,
		},
		// the keys of the deprecated file are validated just like the others.
		"invalid key in the deprecated file": {
			args:    []string{`--ssh-keys-file=` + invalidFile},
			wantErr: "invalid SSH public key on line 1",
		},
		"invalid inline key": {
			args:    []string{`--ssh-keys=not a key`},
			wantErr: "error reading --ssh-keys: invalid SSH public key on line 1",
		},
		// commas separate the options of an authorized_keys line, they must not
		// be mistaken for a separator between keys.
		"inline key with options": {
			args: []string{`--ssh-keys=restrict,pty ` + testPublicKeyA},
			want: []storage.SSHKey{`restrict,pty ` + testPublicKeyA},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			out := &bytes.Buffer{}
			w := format.NewWriter(out)
			flags := parseDatabaseSSHKeys(t, tt.args...)

			keys, err := flags.StorageKeys(&w)
			if tt.wantErr != "" {
				is.ErrorContains(err, tt.wantErr)
				return
			}

			is.NoError(err)
			is.Equal(tt.want, keys)
			if tt.wantWarn == "" {
				is.Empty(out.String())
			} else {
				is.Contains(out.String(), tt.wantWarn)
			}
		})
	}
}

// TestSetIgnoresNilFiles asserts that a nil entry of a repeated file flag does
// not count as passed. It holds nothing to read, so mistaking it for a passed
// flag would make the update commands clear the configured keys instead of
// keeping them.
func TestSetIgnoresNilFiles(t *testing.T) {
	t.Parallel()

	is := require.New(t)

	is.False(AnyFile(nil))
	is.False(AnyFile([]*os.File{nil, nil}))
	is.True(AnyFile([]*os.File{nil, os.Stdin}))

	is.False(DeprecatedKeysFlags{}.Set())
	is.False(DeprecatedKeysFlags{DeprecatedKeysFromFiles: []*os.File{nil}}.Set())
	is.True(DeprecatedKeysFlags{DeprecatedKeys: []string{testPublicKeyA}}.Set())
	is.True(DeprecatedKeysFlags{DeprecatedKeysFromFiles: []*os.File{os.Stdin}}.Set())
}
