package create

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCloudVM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		create  cloudVMCmd
		want    infrastructure.CloudVirtualMachineParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name: "disks",
			create: cloudVMCmd{
				Disks: map[string]resource.Quantity{"a": resource.MustParse("1Gi")},
			},
			want: infrastructure.CloudVirtualMachineParameters{
				Disks: []infrastructure.Disk{
					{Name: "a", Size: resource.MustParse("1Gi")},
				},
			},
		},
		{
			name:   "bootDisk",
			create: cloudVMCmd{BootDiskSize: new(resource.MustParse("1Gi"))},
			want: infrastructure.CloudVirtualMachineParameters{
				BootDisk: &infrastructure.Disk{
					Name: "root", Size: resource.MustParse("1Gi"),
				},
			},
		},
		{
			name:   "reverseDNS",
			create: cloudVMCmd{ReverseDNS: "me.example.com"},
			want: infrastructure.CloudVirtualMachineParameters{
				ReverseDNS: "me.example.com",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			apiClient := test.SetupClient(t)

			if err := tt.create.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("cloudVMCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &infrastructure.CloudVirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected cloudVM to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(created.Spec.ForProvider, tt.want) {
				t.Fatalf("expected CloudVirtualMachine.Spec.ForProvider = %v, got: %v", created.Spec.ForProvider, tt.want)
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

// TestCloudVMPublicKeys asserts that --ssh-keys and --ssh-keys-from-files
// complement each other. Keys given via files used to replace the inline ones.
// It also covers that a file may hold more than one key, that the keys are
// validated no matter which of the flags they come from, and that the
// deprecated --public-keys and --public-keys-from-files still contribute.
func TestCloudVMPublicKeys(t *testing.T) {
	t.Parallel()

	const (
		inlineKey = testPublicKeyA
		fileKey   = testPublicKeyB
	)

	var (
		keyFile      = writeKeyFile(t, "id_ed25519.pub", fileKey+"\n")
		twoKeysFile  = writeKeyFile(t, "authorized_keys", "# my keys\n"+fileKey+"\n\n"+inlineKey+"\n")
		emptyFile    = writeKeyFile(t, "empty.pub", "# no keys in here\n")
		invalidFile  = writeKeyFile(t, "invalid.pub", "not a key\n")
		trailingFile = writeKeyFile(t, "trailing.pub", "  "+fileKey+"  \n\n")
	)

	const deprecationWarning = "--public-keys and --public-keys-from-files are deprecated, use --ssh-keys and --ssh-keys-from-files instead"

	tests := map[string]struct {
		args     []string
		want     []string
		wantWarn string
		wantErr  string
	}{
		"none":   {args: nil, want: nil},
		"inline": {args: []string{`--ssh-keys=` + inlineKey}, want: []string{inlineKey}},
		"file":   {args: []string{`--ssh-keys-from-files=` + keyFile}, want: []string{fileKey}},
		"both": {
			args: []string{`--ssh-keys=` + inlineKey, `--ssh-keys-from-files=` + keyFile},
			want: []string{inlineKey, fileKey},
		},
		"multiple files": {
			args: []string{`--ssh-keys-from-files=` + keyFile, `--ssh-keys-from-files=` + trailingFile},
			want: []string{fileKey, fileKey},
		},
		"multiple keys in one file": {
			args: []string{`--ssh-keys-from-files=` + twoKeysFile},
			want: []string{fileKey, inlineKey},
		},
		"whitespace is trimmed": {
			args: []string{`--ssh-keys-from-files=` + trailingFile},
			want: []string{fileKey},
		},
		// a file may hold nothing but comments, that is only worth a warning as
		// long as at least one key is configured somewhere.
		"file without keys": {
			args:     []string{`--ssh-keys=` + inlineKey, `--ssh-keys-from-files=` + emptyFile},
			want:     []string{inlineKey},
			wantWarn: `no SSH public key found in "` + emptyFile + `"`,
		},
		"inline without keys": {
			args:     []string{`--ssh-keys=# a comment`, `--ssh-keys-from-files=` + keyFile},
			want:     []string{fileKey},
			wantWarn: "no SSH public key found in --ssh-keys",
		},
		"file with an invalid key": {
			args: []string{`--ssh-keys-from-files=` + invalidFile}, wantErr: "invalid SSH public key on line 1",
		},
		"invalid inline key": {
			args: []string{`--ssh-keys=not a key`}, wantErr: "error reading --ssh-keys: invalid SSH public key on line 1",
		},
		"multiple inline keys": {
			args: []string{`--ssh-keys=` + inlineKey, `--ssh-keys=` + fileKey},
			want: []string{inlineKey, fileKey},
		},
		// commas separate the options of an authorized_keys line, they must not
		// be mistaken for a separator between keys.
		"inline key with options": {
			args: []string{`--ssh-keys=restrict,pty ` + inlineKey},
			want: []string{`restrict,pty ` + inlineKey},
		},
		"reports the offending inline key": {
			args:    []string{`--ssh-keys=` + inlineKey, `--ssh-keys=not a key`},
			wantErr: "error reading --ssh-keys: invalid SSH public key on line 2",
		},
		// the deprecated flags keep working, they are merged after the keys of
		// the flags which replace them.
		"deprecated inline": {
			args:     []string{`--public-keys=` + inlineKey},
			want:     []string{inlineKey},
			wantWarn: deprecationWarning,
		},
		"deprecated file": {
			args:     []string{`--public-keys-from-files=` + keyFile},
			want:     []string{fileKey},
			wantWarn: deprecationWarning,
		},
		"deprecated and current": {
			args:     []string{`--public-keys=` + fileKey, `--ssh-keys=` + inlineKey},
			want:     []string{inlineKey, fileKey},
			wantWarn: deprecationWarning,
		},
		"invalid deprecated inline key": {
			args:    []string{`--public-keys=not a key`},
			wantErr: "error reading --public-keys: invalid SSH public key on line 1",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			out := &bytes.Buffer{}
			cmd := parseCloudVM(t, append([]string{`test-cloudvm`}, tt.args...)...)
			cmd.Writer = format.NewWriter(out)

			cloudVM, err := cmd.newCloudVM("default")
			if tt.wantErr != "" {
				is.ErrorContains(err, tt.wantErr)
				return
			}

			is.NoError(err)
			is.Equal(tt.want, cloudVM.Spec.ForProvider.PublicKeys)
			if tt.wantWarn == "" {
				is.Empty(out.String())
			} else {
				is.Contains(out.String(), tt.wantWarn)
			}
		})
	}
}

// TestCloudVMFileFlagsRegression is a regression test for `create cloudvm`
// failing with "error reading cloudconfig file: read <cwd>: is a directory" on
// every invocation that did not pass --cloud-config-from-file.
//
// Giving an *os.File flag a `default:""` makes Kong resolve the empty path to
// the current working directory and open it, so the flag is never nil and the
// nil checks in newCloudVM do not guard anything. The file backed flags must
// therefore stay unset when they are not passed.
//
// The same `default:""` also made Kong decode every occurrence of a repeated
// file flag after the first one to nil, silently dropping those files.
func TestCloudVMFileFlagsRegression(t *testing.T) {
	t.Parallel()

	t.Run("unset", func(t *testing.T) {
		t.Parallel()

		is := require.New(t)

		cmd := parseCloudVM(t, `test-cloudvm`)
		is.Nil(cmd.CloudConfigFromFile)
		is.Empty(cmd.SSHKeysFromFiles)

		cloudVM, err := cmd.newCloudVM("default")
		is.NoError(err)
		is.Empty(cloudVM.Spec.ForProvider.CloudConfig)
		is.Empty(cloudVM.Spec.ForProvider.PublicKeys)
	})

	t.Run("set", func(t *testing.T) {
		t.Parallel()

		is := require.New(t)

		dir := t.TempDir()
		cloudConfig := filepath.Join(dir, "cloud-config.yaml")
		is.NoError(os.WriteFile(cloudConfig, []byte("#cloud-config\n"), 0o600))

		cmd := parseCloudVM(t, `test-cloudvm`,
			`--cloud-config-from-file=`+cloudConfig,
			`--ssh-keys-from-files=`+writeKeyFile(t, "a.pub", testPublicKeyA),
			`--ssh-keys-from-files=`+writeKeyFile(t, "b.pub", testPublicKeyB),
		)

		// every repeated occurrence of the flag has to be decoded, not just
		// the first one.
		is.Len(cmd.SSHKeysFromFiles, 2)
		is.NotContains(cmd.SSHKeysFromFiles, nil)

		cloudVM, err := cmd.newCloudVM("default")
		is.NoError(err)
		is.Contains(cloudVM.Spec.ForProvider.CloudConfig, "#cloud-config")
		is.Equal([]string{testPublicKeyA, testPublicKeyB}, cloudVM.Spec.ForProvider.PublicKeys)
	})
}
