package update

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Valid ed25519 public keys used across the tests of this package.
const (
	testPublicKeyA = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJQQywLL6rNaZTvaomlhlHVvY36Tq7j1yuxJzBHark/V a@example.com`
	testPublicKeyB = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC3bhEbFGMeJwiB7r2GTr/WLWlxrTG9CxzOTf4fM226g b@example.com`
)

func TestCloudVM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		create infrastructure.CloudVirtualMachineParameters
		update cloudVMCmd
		want   infrastructure.CloudVirtualMachineParameters
		// wantUnchanged expects the update to be a no-op, which succeeds
		// without writing the machine.
		wantUnchanged bool
	}{
		{
			name:          "simple",
			wantUnchanged: true,
		},
		{
			name:   "hostname",
			update: cloudVMCmd{Hostname: "a"},
			want:   infrastructure.CloudVirtualMachineParameters{Hostname: "a"},
		},
		{
			name: "turn on",
			create: infrastructure.CloudVirtualMachineParameters{
				PowerState: infrastructure.VirtualMachinePowerState("off"),
			},
			update: cloudVMCmd{On: new(bool(true))},
			want: infrastructure.CloudVirtualMachineParameters{
				PowerState: infrastructure.VirtualMachinePowerState("on"),
			},
		},
		{
			name: "set reverse DNS",
			create: infrastructure.CloudVirtualMachineParameters{
				ReverseDNS: "",
			},
			update: cloudVMCmd{ReverseDNS: "me.example.com"},
			want: infrastructure.CloudVirtualMachineParameters{
				ReverseDNS: "me.example.com",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			tt.update.Writer = format.NewWriter(out)
			tt.update.Name = "test-" + t.Name()

			apiClient := test.SetupClient(t)

			created := test.CloudVirtualMachine(tt.update.Name, apiClient.Project, "nine-es34", tt.create.PowerState)
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("cloudvm create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected cloudvm to exist, got: %s", err)
			}

			updated := &infrastructure.CloudVirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: created.Name, Namespace: created.Namespace}}
			if err := tt.update.Run(t.Context(), apiClient); err != nil {
				t.Errorf("cloudVMCmd.Run() error = %v", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(updated), updated); err != nil {
				t.Fatalf("expected cloudvm to exist, got: %s", err)
			}

			if !reflect.DeepEqual(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected CloudVirtualMachine.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}

			wantOutput := "updated"
			if tt.wantUnchanged {
				wantOutput = "no changes made"
			}
			if !strings.Contains(out.String(), wantOutput) {
				t.Errorf("expected output to contain %q, got: %s", wantOutput, out.String())
			}
			if !strings.Contains(out.String(), tt.update.Name) {
				t.Errorf("expected output to contain %q, got: %s", tt.update.Name, out.String())
			}
		})
	}
}

// parseCloudVM parses args into a cloudVMCmd the same way the real CLI does,
// so that Kong's defaults and mappers are exercised.
func parseCloudVM(t *testing.T, args ...string) *cloudVMCmd {
	t.Helper()

	cmd := &cloudVMCmd{}
	_, err := kong.Must(cmd, kong.BindTo(io.Discard, (*io.Writer)(nil))).Parse(args)
	require.NoError(t, err)

	return cmd
}

// writeKeyFile writes content to a file in a fresh temporary directory and
// returns its path.
func writeKeyFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// TestCloudVMRescuePublicKeys asserts that --rescue-public-keys and
// --rescue-public-keys-from-files complement each other and that the keys are
// validated no matter which of the two flags they come from. Inline keys used
// to be ignored entirely.
func TestCloudVMRescuePublicKeys(t *testing.T) {
	t.Parallel()

	var (
		keyFile      = writeKeyFile(t, "id_ed25519.pub", testPublicKeyB+"\n")
		twoKeysFile  = writeKeyFile(t, "authorized_keys", "# my keys\n"+testPublicKeyB+"\n\n"+testPublicKeyA+"\n")
		emptyFile    = writeKeyFile(t, "empty.pub", "# no keys in here\n")
		invalidFile  = writeKeyFile(t, "invalid.pub", "not a key\n")
		trailingFile = writeKeyFile(t, "trailing.pub", "  "+testPublicKeyB+"  \n\n")
	)

	tests := map[string]struct {
		args     []string
		rescue   *infrastructure.CloudVirtualMachineRescue
		want     *infrastructure.CloudVirtualMachineRescue
		wantWarn string
		wantErr  string
	}{
		"none": {args: nil},
		"inline": {
			args: []string{`--rescue-public-keys=` + testPublicKeyA},
			want: &infrastructure.CloudVirtualMachineRescue{PublicKeys: []string{testPublicKeyA}},
		},
		"file": {
			args: []string{`--rescue-public-keys-from-files=` + keyFile},
			want: &infrastructure.CloudVirtualMachineRescue{PublicKeys: []string{testPublicKeyB}},
		},
		"both": {
			args: []string{`--rescue-public-keys=` + testPublicKeyA, `--rescue-public-keys-from-files=` + keyFile},
			want: &infrastructure.CloudVirtualMachineRescue{PublicKeys: []string{testPublicKeyA, testPublicKeyB}},
		},
		"multiple files": {
			args: []string{`--rescue-public-keys-from-files=` + keyFile, `--rescue-public-keys-from-files=` + trailingFile},
			want: &infrastructure.CloudVirtualMachineRescue{PublicKeys: []string{testPublicKeyB, testPublicKeyB}},
		},
		"multiple keys in one file": {
			args: []string{`--rescue-public-keys-from-files=` + twoKeysFile},
			want: &infrastructure.CloudVirtualMachineRescue{PublicKeys: []string{testPublicKeyB, testPublicKeyA}},
		},
		// the keys replace the ones which are already set, while everything
		// else about the rescue configuration is kept.
		"replaces existing keys": {
			args:   []string{`--rescue-public-keys=` + testPublicKeyA},
			rescue: &infrastructure.CloudVirtualMachineRescue{Enabled: true, PublicKeys: []string{testPublicKeyB}},
			want:   &infrastructure.CloudVirtualMachineRescue{Enabled: true, PublicKeys: []string{testPublicKeyA}},
		},
		"keeps existing keys when unset": {
			args:   nil,
			rescue: &infrastructure.CloudVirtualMachineRescue{Enabled: true, PublicKeys: []string{testPublicKeyB}},
			want:   &infrastructure.CloudVirtualMachineRescue{Enabled: true, PublicKeys: []string{testPublicKeyB}},
		},
		// passing the flag without a value is how the keys are removed, the
		// rest of the rescue configuration is kept.
		"clears existing keys": {
			args:   []string{`--rescue-public-keys=`},
			rescue: &infrastructure.CloudVirtualMachineRescue{Enabled: true, PublicKeys: []string{testPublicKeyB}},
			want:   &infrastructure.CloudVirtualMachineRescue{Enabled: true},
		},
		// nothing to remove, the rescue configuration must not be allocated
		// just to hold no keys.
		"clears keys of an unconfigured rescue": {
			args: []string{`--rescue-public-keys=`},
			want: nil,
		},
		"clears before adding": {
			args:   []string{`--rescue-public-keys=`, `--rescue-public-keys-from-files=` + keyFile},
			rescue: &infrastructure.CloudVirtualMachineRescue{Enabled: true, PublicKeys: []string{testPublicKeyA}},
			want:   &infrastructure.CloudVirtualMachineRescue{Enabled: true, PublicKeys: []string{testPublicKeyB}},
		},
		// a source may hold nothing but comments, that is only worth a warning.
		"file without keys": {
			args:     []string{`--rescue-public-keys=` + testPublicKeyA, `--rescue-public-keys-from-files=` + emptyFile},
			want:     &infrastructure.CloudVirtualMachineRescue{PublicKeys: []string{testPublicKeyA}},
			wantWarn: `no SSH public key found in "` + emptyFile + `"`,
		},
		"inline without keys": {
			args:     []string{`--rescue-public-keys=# a comment`, `--rescue-public-keys-from-files=` + keyFile},
			want:     &infrastructure.CloudVirtualMachineRescue{PublicKeys: []string{testPublicKeyB}},
			wantWarn: "no SSH public key found in --rescue-public-keys",
		},
		"invalid inline key": {
			args:    []string{`--rescue-public-keys=not a key`},
			wantErr: "error reading --rescue-public-keys: invalid SSH public key on line 1",
		},
		"file with an invalid key": {
			args:    []string{`--rescue-public-keys-from-files=` + invalidFile},
			wantErr: "invalid SSH public key on line 1",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			cloudVM := &infrastructure.CloudVirtualMachine{}
			cloudVM.Spec.ForProvider.Rescue = tt.rescue

			out := &bytes.Buffer{}
			cmd := parseCloudVM(t, append([]string{`test-cloudvm`}, tt.args...)...)
			cmd.Writer = format.NewWriter(out)

			err := cmd.applyUpdates(cloudVM)
			if tt.wantErr != "" {
				is.ErrorContains(err, tt.wantErr)
				return
			}

			is.NoError(err)
			is.Equal(tt.want, cloudVM.Spec.ForProvider.Rescue)
			if tt.wantWarn == "" {
				is.Empty(out.String())
			} else {
				is.Contains(out.String(), tt.wantWarn)
			}
		})
	}
}
