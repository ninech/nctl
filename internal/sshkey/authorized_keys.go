// Package sshkey reads SSH public keys from flags and files. It provides the
// kong flag structs the commands share, validates the keys they are passed in
// the authorized_keys format and converts them to the API types.
package sshkey

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"golang.org/x/crypto/ssh"

	"github.com/ninech/nctl/internal/format"
)

// ParseAuthorizedKeys reads SSH public keys from r. Every key is expected on
// its own line in the SSH format defined in RFC4253, blank lines and lines
// prefixed with # are ignored.
//
// The keys are validated with [ssh.ParseAuthorizedKey] but returned as they
// were read, only stripped of surrounding whitespace, so that key comments and
// options are preserved.
func ParseAuthorizedKeys(r io.Reader) ([]string, error) {
	var keys []string

	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		key := strings.TrimSpace(scanner.Text())
		if key == "" || strings.HasPrefix(key, "#") {
			continue
		}

		if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key)); err != nil {
			return nil, fmt.Errorf("invalid SSH public key on line %d: %w", line, err)
		}

		keys = append(keys, key)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

// ReadAuthorizedKeys returns the validated SSH public keys of values, followed
// by the keys read from files, in that order. flag names the flag values were
// passed to and is only used to report errors and warnings. The files are
// closed, they are read exactly once.
//
// A source which holds no key at all is not an error, but it is warned about on
// w as it is most likely not what the user intended. Values which hold nothing
// but whitespace are not warned about, as that is how a configured list of keys
// is cleared.
func ReadAuthorizedKeys(w *format.Writer, flag string, values []string, files []*os.File) ([]string, error) {
	joined := strings.Join(values, "\n")

	keys, err := ParseAuthorizedKeys(strings.NewReader(joined))
	if err != nil {
		return nil, fmt.Errorf("error reading --%s: %w", flag, err)
	}
	if strings.TrimSpace(joined) != "" && len(keys) == 0 {
		w.Warningf("no SSH public key found in --%s", flag)
	}

	for _, file := range files {
		if file == nil {
			continue
		}

		fileKeys, err := readAuthorizedKeysFile(file)
		if err != nil {
			return nil, err
		}
		if len(fileKeys) == 0 {
			w.Warningf("no SSH public key found in %q", file.Name())
		}
		keys = append(keys, fileKeys...)
	}

	return keys, nil
}

// readAuthorizedKeysFile reads the validated SSH public keys of file and closes
// it. It is a function of its own so that every file is closed as soon as it was
// read instead of only when the surrounding loop is done.
func readAuthorizedKeysFile(file *os.File) ([]string, error) {
	defer file.Close()

	keys, err := ParseAuthorizedKeys(file)
	if err != nil {
		return nil, fmt.Errorf("error reading public keys file %q: %w", file.Name(), err)
	}

	return keys, nil
}

// AnyFile reports whether files holds at least one file Kong decoded. A
// repeated file flag may hold nil entries, those do not count as passed as
// there is nothing to read from them.
func AnyFile(files []*os.File) bool {
	return slices.ContainsFunc(files, func(file *os.File) bool { return file != nil })
}

// StorageKeysWithDeprecatedFile converts keys to the type of the storage API,
// with the keys of the deprecated --ssh-keys-file appended. Its use is warned
// about on w, pointing at the flag which replaces it. A nil file was not passed
// and contributes no keys.
//
// It is shared by the database create and update commands, which declare the
// same three flags but differ in how the flags they replace are declared.
func StorageKeysWithDeprecatedFile(w *format.Writer, keys []string, deprecatedFile *os.File) ([]storage.SSHKey, error) {
	if deprecatedFile != nil {
		w.Warningf("--ssh-keys-file is deprecated, use --ssh-keys-from-files instead")

		fileKeys, err := ReadAuthorizedKeys(w, "ssh-keys-file", nil, []*os.File{deprecatedFile})
		if err != nil {
			return nil, err
		}
		keys = append(keys, fileKeys...)
	}

	return StorageSSHKeys(keys), nil
}

// StorageSSHKeys converts keys to the type of the storage API. An empty list of
// keys converts to nil, so that callers can tell "no keys were given" from "the
// configured keys are to be cleared" through the flags instead of the result.
func StorageSSHKeys(keys []string) []storage.SSHKey {
	if len(keys) == 0 {
		return nil
	}

	sshKeys := make([]storage.SSHKey, len(keys))
	for i, key := range keys {
		sshKeys[i] = storage.SSHKey(key)
	}

	return sshKeys
}
