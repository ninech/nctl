package exec

import (
	"context"
	"strings"
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestKVSCmd(t *testing.T) {
	t.Parallel()

	const (
		kvsName  = "mykvs"
		kvsFQDN  = "mykvs.example.com"
		kvsToken = "supersecrettoken"
	)

	cidr := []meta.IPv4CIDR{"203.0.113.5/32"}
	pubNet := true

	ready := test.KeyValueStore(kvsName, test.DefaultProject, "nine-es34")
	ready.Status.AtProvider.FQDN = kvsFQDN
	ready.Spec.ForProvider.AllowedCIDRs = []meta.IPv4CIDR{"10.0.0.1/32"}
	ready.Spec.ForProvider.PublicNetworkingEnabled = &pubNet

	pubNetFalse := false
	pubNetDisabled := test.KeyValueStore("no-public", test.DefaultProject, "nine-es34")
	pubNetDisabled.Status.AtProvider.FQDN = "no-public.example.com"
	pubNetDisabled.Spec.ForProvider.PublicNetworkingEnabled = &pubNetFalse
	pubNetDisabled.Spec.ForProvider.AllowedCIDRs = []meta.IPv4CIDR{}

	notReady := test.KeyValueStore("notready", test.DefaultProject, "nine-es34")

	// KVS secret: single key with auth token as value.
	secret := testSecret(kvsName, test.DefaultProject, "token", kvsToken)

	_, notFoundCmd := testDatabaseCmd("doesnotexist", &cidr)
	_, notReadyCmd := testDatabaseCmd("notready", &cidr)
	alreadyCap, alreadyPresentCmd := testDatabaseCmd(kvsName, &[]meta.IPv4CIDR{"10.0.0.1/32"})
	_, newCidrCmd := testDatabaseCmdConfirmed(kvsName, &cidr, true)
	_, pubNetDisabledCmd := testDatabaseCmdConfirmed("no-public", &cidr, true)
	tokenCap, tokenCmd := testDatabaseCmd(kvsName, &[]meta.IPv4CIDR{"10.0.0.1/32"})

	tests := []struct {
		name        string
		cmd         kvsCmd
		cap         *capturingCmd
		wantErr     bool
		errContains string
		wantUpdate  bool
		checkArgs   func(t *testing.T, args []string)
	}{
		{
			name:    "resource not found",
			cmd:     kvsCmd{serviceCmd: notFoundCmd},
			wantErr: true,
		},
		{
			name:        "resource not ready",
			cmd:         kvsCmd{serviceCmd: notReadyCmd},
			wantErr:     true,
			errContains: "not ready",
		},
		{
			name: "cidr already present skips update",
			cmd:  kvsCmd{serviceCmd: alreadyPresentCmd},
			cap:  alreadyCap,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				if !strings.Contains(strings.Join(args, " "), kvsFQDN) {
					t.Errorf("expected FQDN %q in args %v", kvsFQDN, args)
				}
			},
		},
		{
			name:       "new cidr triggers update",
			cmd:        kvsCmd{serviceCmd: newCidrCmd},
			wantUpdate: true,
		},
		{
			name:        "public networking disabled returns error",
			cmd:         kvsCmd{serviceCmd: pubNetDisabledCmd},
			wantErr:     true,
			errContains: "networking is disabled",
		},
		{
			name: "token appears in args",
			cmd:  kvsCmd{serviceCmd: tokenCmd},
			cap:  tokenCap,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				if !strings.Contains(strings.Join(args, " "), kvsToken) {
					t.Errorf("expected token in args %v", args)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updateCalled := false
			apiClient := test.SetupClient(t,
				test.WithObjects(ready, notReady, pubNetDisabled, secret),
				test.WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, c runtimeclient.WithWatch, obj runtimeclient.Object, opts ...runtimeclient.UpdateOption) error {
						updateCalled = true
						return c.Update(ctx, obj, opts...)
					},
				}),
			)

			err := tc.cmd.Run(t.Context(), apiClient)

			if (err != nil) != tc.wantErr {
				t.Fatalf("Run() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.errContains != "" && (err == nil || !strings.Contains(err.Error(), tc.errContains)) {
				t.Errorf("expected error containing %q, got %v", tc.errContains, err)
			}
			if tc.wantUpdate && !updateCalled {
				t.Error("expected Update to be called for CIDR addition")
			}
			if !tc.wantErr && tc.checkArgs != nil {
				tc.checkArgs(t, tc.cap.args)
			}
			if tc.wantUpdate {
				kvs := &storage.KeyValueStore{}
				if err := apiClient.Get(t.Context(), api.ObjectName(ready), kvs); err != nil {
					t.Fatalf("getting kvs: %v", err)
				}
				if !cidrsPresent(kvs.Spec.ForProvider.AllowedCIDRs, cidr) {
					t.Errorf("expected CIDR %v to be added, got %v", cidr, kvs.Spec.ForProvider.AllowedCIDRs)
				}
			}
		})
	}
}
