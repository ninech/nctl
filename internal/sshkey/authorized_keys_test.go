package sshkey

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// openKeyFile writes content to a file in a fresh temporary directory
// and opens it for reading, the way Kong decodes a file flag.
func openKeyFile(t *testing.T, name, content string) *os.File {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	file, err := os.Open(path)
	require.NoError(t, err)

	return file
}

// TestReadAuthorizedKeys asserts the order of the keys,
// that every source is validated,
// that the flag name is spelled out in the errors
// and that a source without any key is warned about,
// except for values which only clear the keys.
func TestReadAuthorizedKeys(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		values   []string
		files    map[string]string
		want     []string
		wantWarn string
		wantErr  string
	}{
		"none": {},
		"values before files": {
			values: []string{testPublicKeyA},
			files:  map[string]string{"id_ed25519.pub": testPublicKeyB + "\n"},
			want:   []string{testPublicKeyA, testPublicKeyB},
		},
		"newline separated values": {
			values: []string{testPublicKeyA + "\n" + testPublicKeyB},
			want:   []string{testPublicKeyA, testPublicKeyB},
		},
		"empty value clears without a warning": {
			values: []string{""},
		},
		"value without a key": {
			values:   []string{"# just a comment"},
			wantWarn: `no SSH public key found in --rescue-ssh-keys`,
		},
		"file without a key": {
			files:    map[string]string{"empty.pub": "# nothing\n"},
			wantWarn: `no SSH public key found in "`,
		},
		"invalid value": {
			values:  []string{"not a key"},
			wantErr: "error reading --rescue-ssh-keys: invalid SSH public key on line 1",
		},
		"invalid file": {
			files:   map[string]string{"invalid.pub": "not a key\n"},
			wantErr: `error reading public keys file "`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			// a nil entry is what Kong leaves behind for a repeated file flag which was not passed,
			// it must be skipped.
			files := []*os.File{nil}
			for name, content := range tt.files {
				files = append(files, openKeyFile(t, name, content))
			}

			out := &bytes.Buffer{}
			w := format.NewWriter(out)

			keys, err := ReadAuthorizedKeys(&w, "rescue-ssh-keys", tt.values, files)
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

			// the files are read exactly once and closed afterwards.
			for _, file := range files[1:] {
				is.ErrorIs(file.Close(), os.ErrClosed)
			}
		})
	}
}

// TestSetIgnoresNilFiles asserts that a nil entry of a repeated file flag does not count as passed.
// It holds nothing to read,
// so mistaking it for a passed flag would make the update commands clear the configured keys
// instead of keeping them.
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

// TestStorageKeysWithDeprecatedFile asserts that the deprecated file adds to the keys instead of replacing them,
// is warned about and validated,
// and that no keys at all convert to nil rather than to an empty slice.
func TestStorageKeysWithDeprecatedFile(t *testing.T) {
	t.Parallel()

	const deprecationWarning = "--ssh-keys-file is deprecated, use --ssh-keys-from-files instead"

	t.Run("without the file", func(t *testing.T) {
		t.Parallel()
		is := require.New(t)

		out := &bytes.Buffer{}
		w := format.NewWriter(out)

		keys, err := StorageKeysWithDeprecatedFile(&w, nil, nil)
		is.NoError(err)
		is.Nil(keys)

		keys, err = StorageKeysWithDeprecatedFile(&w, []string{testPublicKeyA}, nil)
		is.NoError(err)
		is.Equal([]storage.SSHKey{testPublicKeyA}, keys)
		is.Empty(out.String())
	})

	t.Run("with the file", func(t *testing.T) {
		t.Parallel()
		is := require.New(t)

		out := &bytes.Buffer{}
		w := format.NewWriter(out)

		keys, err := StorageKeysWithDeprecatedFile(&w, []string{testPublicKeyA}, openKeyFile(t, "authorized_keys", testPublicKeyB+"\n"))
		is.NoError(err)
		is.Equal([]storage.SSHKey{testPublicKeyA, testPublicKeyB}, keys)
		is.Contains(out.String(), deprecationWarning)
	})

	t.Run("with an invalid file", func(t *testing.T) {
		t.Parallel()
		is := require.New(t)

		w := format.NewWriter(&bytes.Buffer{})

		_, err := StorageKeysWithDeprecatedFile(&w, nil, openKeyFile(t, "invalid.pub", "not a key\n"))
		is.ErrorContains(err, "invalid SSH public key on line 1")
	})
}
