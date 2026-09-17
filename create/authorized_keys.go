package create

import (
	"os"

	storage "github.com/ninech/apis/storage/v1alpha1"

	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/sshkey"
)

// DatabaseSSHKeysFlags declares the SSH public key flags of the database create
// commands, including the deprecated --ssh-keys-file which the repeatable
// --ssh-keys-from-files replaces.
type DatabaseSSHKeysFlags struct {
	sshkey.SSHKeysFlags

	// Deprecated Flags
	SSHKeysFile *os.File `hidden:"" completion-predictor:"local:file" help:"Deprecated, use --ssh-keys-from-files instead."`
}

// StorageKeys returns the validated SSH public keys of all three flags, in the
// order --ssh-keys, --ssh-keys-from-files and --ssh-keys-file.
func (f DatabaseSSHKeysFlags) StorageKeys(w *format.Writer) ([]storage.SSHKey, error) {
	keys, err := f.Keys(w, "")
	if err != nil {
		return nil, err
	}

	return sshkey.StorageKeysWithDeprecatedFile(w, keys, f.SSHKeysFile)
}
