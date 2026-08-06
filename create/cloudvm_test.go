package create

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
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
	_, err := kong.Must(cmd, CloudVMKongVars(), kong.BindTo(io.Discard, (*io.Writer)(nil))).Parse(args)
	require.NoError(t, err)

	return cmd
}

// TestCloudVMPublicKeys asserts that --public-keys and --public-keys-from-files
// complement each other. Keys given via files used to replace the inline ones.
func TestCloudVMPublicKeys(t *testing.T) {
	t.Parallel()

	const (
		inlineKey = "ssh-ed25519 AAAAC3Nzinline inline"
		fileKey   = "ssh-ed25519 AAAAC3Nzfile file"
	)

	keyFile := filepath.Join(t.TempDir(), "id_ed25519.pub")
	require.NoError(t, os.WriteFile(keyFile, []byte(fileKey), 0o600))

	tests := map[string]struct {
		args []string
		want []string
	}{
		"none":   {args: nil, want: nil},
		"inline": {args: []string{`--public-keys=` + inlineKey}, want: []string{inlineKey}},
		"file":   {args: []string{`--public-keys-from-files=` + keyFile}, want: []string{fileKey}},
		"both": {
			args: []string{`--public-keys=` + inlineKey, `--public-keys-from-files=` + keyFile},
			want: []string{inlineKey, fileKey},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			cloudVM, err := parseCloudVM(t, append([]string{`test-cloudvm`}, tt.args...)...).newCloudVM("default")
			is.NoError(err)
			is.Equal(tt.want, cloudVM.Spec.ForProvider.PublicKeys)
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
func TestCloudVMFileFlagsRegression(t *testing.T) {
	t.Parallel()

	t.Run("unset", func(t *testing.T) {
		t.Parallel()

		is := require.New(t)

		cmd := parseCloudVM(t, `test-cloudvm`)
		is.Nil(cmd.CloudConfigFromFile)
		is.Empty(cmd.PublicKeysFromFiles)

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
		publicKey := filepath.Join(dir, "id_ed25519.pub")
		is.NoError(os.WriteFile(publicKey, []byte("ssh-ed25519 AAAAC3Nz test\n"), 0o600))

		cmd := parseCloudVM(t, `test-cloudvm`,
			`--cloud-config-from-file=`+cloudConfig,
			`--public-keys-from-files=`+publicKey,
		)

		cloudVM, err := cmd.newCloudVM("default")
		is.NoError(err)
		is.Contains(cloudVM.Spec.ForProvider.CloudConfig, "#cloud-config")
		is.Len(cloudVM.Spec.ForProvider.PublicKeys, 1)
		is.Contains(cloudVM.Spec.ForProvider.PublicKeys[0], "ssh-ed25519 AAAAC3Nz test")
	})
}
