package sshkey

import (
	"os"

	"github.com/ninech/nctl/internal/format"
)

// SSHKeysFlags declares the --ssh-keys and --ssh-keys-from-files flags. Embed
// it with a prefix to scope the flags to a part of a resource and set
// ssh_keys_purpose to the clause which says what the keys are used for, it
// completes the help of both flags:
//
//	sshkey.SSHKeysFlags `prefix:"rescue-" set:"ssh_keys_purpose=to connect while booted into rescue"`
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
	SSHKeysFromFiles []*os.File `placeholder:"~/.ssh/id_ed25519.pub" completion-predictor:"local:file" help:"Files holding SSH public keys ${ssh_keys_purpose=to connect to the resource}. Empty lines and lines prefixed with # are ignored."`
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
//	sshkey.DeprecatedKeysFlags `prefix:"public-"`
//
// The flags are hidden, they only exist to keep the previous spelling working.
// Unlike [SSHKeysFlags] they do not set sep:"none", so that values which used to
// be split on commas keep being split the same way. The fields carry a name tag
// so that they can be told apart from the fields of [SSHKeysFlags] wherever both
// are embedded, the flags are still registered as --<prefix>keys and
// --<prefix>keys-from-files.
type DeprecatedKeysFlags struct {
	DeprecatedKeys          []string   `name:"keys" hidden:"" help:"Deprecated, use --ssh-keys instead."`
	DeprecatedKeysFromFiles []*os.File `name:"keys-from-files" hidden:"" completion-predictor:"local:file" help:"Deprecated, use --ssh-keys-from-files instead."`
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
