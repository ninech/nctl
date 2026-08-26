package update

import (
	"os"

	storage "github.com/ninech/apis/storage/v1alpha1"

	"github.com/ninech/nctl/create"
	"github.com/ninech/nctl/internal/format"
)

// OptionalSSHKeysFlags is [create.SSHKeysFlags] for the update commands, where
// a flag which was not passed has to be told apart from one which was passed an
// empty value in order to clear the configured keys. It is embedded and
// described just like [create.SSHKeysFlags].
type OptionalSSHKeysFlags struct {
	// sep:"none" keeps Kong from splitting a value on commas, which would tear
	// apart the options of an authorized_keys line. Repeat the flag or separate
	// the keys by newlines to pass more than one.
	SSHKeys          []string   `sep:"none" placeholder:"ssh-ed25519 AAAA..." help:"SSH public keys ${ssh_keys_purpose=to connect to the resource}. The keys are expected to be in SSH format as defined in RFC4253. Repeat the flag to pass more than one key, pass it an empty value to remove all configured keys."`
	SSHKeysFromFiles []*os.File `placeholder:"~/.ssh/id_ed25519.pub" completion-predictor:"local:file" help:"Files holding SSH public keys ${ssh_keys_purpose=to connect to the resource}. Empty lines and lines prefixed with # are ignored."`
}

// SSHKeysSet reports whether one of the flags was passed at all. If it is false
// the keys which are already configured have to be kept, while SSHKeysSet
// together with no keys asks for the configured keys to be removed, which is
// what passing --ssh-keys= does: sep:"none" makes Kong decode the empty value
// into a single empty entry, telling the flag apart from one which was never
// passed.
func (f OptionalSSHKeysFlags) SSHKeysSet() bool {
	return len(f.SSHKeys) != 0 || create.AnyFile(f.SSHKeysFromFiles)
}

// Keys returns the validated SSH public keys of --<prefix>ssh-keys followed by
// those of --<prefix>ssh-keys-from-files, in that order.
func (f OptionalSSHKeysFlags) Keys(w *format.Writer, prefix string) ([]string, error) {
	return create.ReadAuthorizedKeys(w, prefix+"ssh-keys", f.SSHKeys, f.SSHKeysFromFiles)
}

// DatabaseSSHKeysFlags declares the SSH public key flags of the database update
// commands, including the deprecated --ssh-keys-file which the repeatable
// --ssh-keys-from-files replaces.
type DatabaseSSHKeysFlags struct {
	OptionalSSHKeysFlags

	// Deprecated Flags
	SSHKeysFile *os.File `hidden:"" completion-predictor:"local:file" help:"Deprecated, use --ssh-keys-from-files instead."`
}

// SSHKeysSet reports whether one of the flags was passed at all.
func (f DatabaseSSHKeysFlags) SSHKeysSet() bool {
	return f.OptionalSSHKeysFlags.SSHKeysSet() || f.SSHKeysFile != nil
}

// StorageKeys returns the validated SSH public keys of all three flags, in the
// order --ssh-keys, --ssh-keys-from-files and --ssh-keys-file.
func (f DatabaseSSHKeysFlags) StorageKeys(w *format.Writer) ([]storage.SSHKey, error) {
	keys, err := f.Keys(w, "")
	if err != nil {
		return nil, err
	}

	return create.StorageKeysWithDeprecatedFile(w, keys, f.SSHKeysFile)
}
