package create

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

// SSHKeysFlags declares the --ssh-keys and --ssh-keys-from-files flags. Embed
// it with a prefix to scope the flags to a part of a resource and set
// ssh_keys_purpose to the clause which says what the keys are used for, it
// completes the help of both flags:
//
//	SSHKeysFlags `prefix:"rescue-" set:"ssh_keys_purpose=to connect while booted into rescue"`
//
// The same prefix has to be passed to [SSHKeysFlags.Keys], as Kong does not
// tell a flag which name it ended up being registered under. A mismatch between
// the two only shows in the errors and warnings, so every command embedding the
// flags is expected to have a test which parses real arguments through Kong and
// asserts the flag names those messages spell out.
type SSHKeysFlags struct {
	// sep:"none" keeps Kong from splitting a value on commas, which would tear
	// apart the options of an authorized_keys line. Repeat the flag or separate
	// the keys by newlines to pass more than one.
	SSHKeys          []string   `sep:"none" placeholder:"ssh-ed25519 AAAA..." help:"SSH public keys ${ssh_keys_purpose=to connect to the resource}. The keys are expected to be in SSH format as defined in RFC4253. Repeat the flag to pass more than one key."`
	SSHKeysFromFiles []*os.File `placeholder:"~/.ssh/id_ed25519.pub" completion-predictor:"file" help:"Files holding SSH public keys ${ssh_keys_purpose=to connect to the resource}. Empty lines and lines prefixed with # are ignored."`
}

// Keys returns the validated SSH public keys of --<prefix>ssh-keys followed by
// those of --<prefix>ssh-keys-from-files, in that order.
func (f SSHKeysFlags) Keys(w *format.Writer, prefix string) ([]string, error) {
	return ReadAuthorizedKeys(w, prefix+"ssh-keys", f.SSHKeys, f.SSHKeysFromFiles)
}

// DeprecatedKeysFlags declares the --keys and --keys-from-files flags which
// [SSHKeysFlags] replaces. It has to be embedded with the prefix the flags were
// registered under before the rename:
//
//	DeprecatedKeysFlags `prefix:"public-"`
//
// The flags are hidden, they only exist to keep the previous spelling working.
// Unlike [SSHKeysFlags] they do not set sep:"none", so that values which used to
// be split on commas keep being split the same way. The fields carry a name tag
// so that they can be told apart from the fields of [SSHKeysFlags] wherever both
// are embedded, the flags are still registered as --<prefix>keys and
// --<prefix>keys-from-files.
type DeprecatedKeysFlags struct {
	DeprecatedKeys          []string   `name:"keys" hidden:"" help:"Deprecated, use --ssh-keys instead."`
	DeprecatedKeysFromFiles []*os.File `name:"keys-from-files" hidden:"" completion-predictor:"file" help:"Deprecated, use --ssh-keys-from-files instead."`
}

// Set reports whether one of the deprecated flags was passed. Unlike the flags
// of [SSHKeysFlags] they cannot be used to clear a configured list of keys.
func (f DeprecatedKeysFlags) Set() bool {
	return len(f.DeprecatedKeys) != 0 || AnyFile(f.DeprecatedKeysFromFiles)
}

// Keys returns the validated SSH public keys of the deprecated flags registered
// under prefix, or nil if none of them was passed. Their use is warned about on
// w, pointing at the flags which [SSHKeysFlags] registered under replacement.
func (f DeprecatedKeysFlags) Keys(w *format.Writer, prefix, replacement string) ([]string, error) {
	if !f.Set() {
		return nil, nil
	}

	w.Warningf(
		"--%[1]skeys and --%[1]skeys-from-files are deprecated, use --%[2]sssh-keys and --%[2]sssh-keys-from-files instead",
		prefix, replacement,
	)

	return ReadAuthorizedKeys(w, prefix+"keys", f.DeprecatedKeys, f.DeprecatedKeysFromFiles)
}

// DatabaseSSHKeysFlags declares the SSH public key flags of the database create
// commands, including the deprecated --ssh-keys-file which the repeatable
// --ssh-keys-from-files replaces.
type DatabaseSSHKeysFlags struct {
	SSHKeysFlags

	// Deprecated Flags
	SSHKeysFile *os.File `hidden:"" completion-predictor:"file" help:"Deprecated, use --ssh-keys-from-files instead."`
}

// StorageKeys returns the validated SSH public keys of all three flags, in the
// order --ssh-keys, --ssh-keys-from-files and --ssh-keys-file.
func (f DatabaseSSHKeysFlags) StorageKeys(w *format.Writer) ([]storage.SSHKey, error) {
	keys, err := f.Keys(w, "")
	if err != nil {
		return nil, err
	}

	return StorageKeysWithDeprecatedFile(w, keys, f.SSHKeysFile)
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
