package update

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/alecthomas/kong"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/stretchr/testify/require"

	"github.com/ninech/nctl/internal/format"
)

// TestOptionalSSHKeysFlagsSet asserts how the flags report whether they were
// passed, which is what tells "keep the configured keys" apart from "clear
// them". A nil entry of a repeated file flag holds nothing to read, so it must
// not count as passed.
func TestOptionalSSHKeysFlagsSet(t *testing.T) {
	t.Parallel()

	is := require.New(t)

	is.False(OptionalSSHKeysFlags{}.SSHKeysSet())
	is.False(OptionalSSHKeysFlags{SSHKeysFromFiles: []*os.File{nil}}.SSHKeysSet())
	is.True(OptionalSSHKeysFlags{SSHKeysFromFiles: []*os.File{os.Stdin}}.SSHKeysSet())
	// the flag passed an empty value, which is how the keys are cleared.
	is.True(OptionalSSHKeysFlags{SSHKeys: []string{""}}.SSHKeysSet())
	is.True(OptionalSSHKeysFlags{SSHKeys: []string{testPublicKeyA}}.SSHKeysSet())

	is.False(DatabaseSSHKeysFlags{}.SSHKeysSet())
	is.False(DatabaseSSHKeysFlags{
		OptionalSSHKeysFlags: OptionalSSHKeysFlags{SSHKeysFromFiles: []*os.File{nil}},
	}.SSHKeysSet())
	is.True(DatabaseSSHKeysFlags{SSHKeysFile: os.Stdin}.SSHKeysSet())
}

// TestOptionalSSHKeysFlagsDecoding pins how Kong decodes --ssh-keys, which is
// what [OptionalSSHKeysFlags.Set] relies on to tell "keep the configured keys"
// apart from "clear them".
//
// A repeated flag has to accumulate. The flag used to be declared as a
// *[]string, where Kong decodes every occurrence into a freshly allocated slice
// which replaces the previous one, silently dropping all but the last key.
func TestOptionalSSHKeysFlagsDecoding(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args     []string
		wantSet  bool
		wantKeys []string
	}{
		"unset": {},
		"one key": {
			args:     []string{`--ssh-keys=` + testPublicKeyA},
			wantSet:  true,
			wantKeys: []string{testPublicKeyA},
		},
		"repeated": {
			args:     []string{`--ssh-keys=` + testPublicKeyA, `--ssh-keys=` + testPublicKeyB},
			wantSet:  true,
			wantKeys: []string{testPublicKeyA, testPublicKeyB},
		},
		"newline separated": {
			args:     []string{`--ssh-keys=` + testPublicKeyA + "\n" + testPublicKeyB},
			wantSet:  true,
			wantKeys: []string{testPublicKeyA, testPublicKeyB},
		},
		// an empty value decodes into a single empty entry rather than into no
		// entry at all, which is what makes it tell itself apart from a flag
		// which was never passed.
		"empty value": {
			args:    []string{`--ssh-keys=`},
			wantSet: true,
		},
		"repeated after an empty value": {
			args:     []string{`--ssh-keys=`, `--ssh-keys=` + testPublicKeyA},
			wantSet:  true,
			wantKeys: []string{testPublicKeyA},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			flags := &struct{ OptionalSSHKeysFlags }{}
			_, err := kong.Must(flags).Parse(tt.args)
			is.NoError(err)

			is.Equal(tt.wantSet, flags.SSHKeysSet())

			w := format.NewWriter(io.Discard)
			keys, err := flags.Keys(&w, "")
			is.NoError(err)
			is.Equal(tt.wantKeys, keys)
		})
	}
}

// TestCloudVMRescueNilKeyFileKeepsKeys asserts that a nil entry of
// --rescue-ssh-keys-from-files keeps the configured keys. There is nothing to
// read from it, so treating it as a passed flag would clear them instead.
func TestCloudVMRescueNilKeyFileKeepsKeys(t *testing.T) {
	t.Parallel()

	is := require.New(t)

	cloudVM := &infrastructure.CloudVirtualMachine{}
	cloudVM.Spec.ForProvider.Rescue = &infrastructure.CloudVirtualMachineRescue{
		Enabled:    true,
		PublicKeys: []string{testPublicKeyA},
	}

	out := &bytes.Buffer{}
	cmd := cloudVMCmd{}
	cmd.Writer = format.NewWriter(out)
	cmd.SSHKeysFromFiles = []*os.File{nil}

	is.NoError(cmd.applyUpdates(cloudVM))
	is.Equal(
		&infrastructure.CloudVirtualMachineRescue{Enabled: true, PublicKeys: []string{testPublicKeyA}},
		cloudVM.Spec.ForProvider.Rescue,
	)
	is.Empty(out.String())
}
