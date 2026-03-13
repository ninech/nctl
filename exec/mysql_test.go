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

func TestMySQLCmd(t *testing.T) {
	t.Parallel()

	const (
		myName = "mymy"
		myFQDN = "mymy.example.com"
		myUser = "root"
		myPass = "rootpass"
	)

	cidr := []meta.IPv4CIDR{"203.0.113.5/32"}

	ready := test.MySQL(myName, test.DefaultProject, "nine-es34")
	ready.Status.AtProvider.FQDN = myFQDN
	ready.Spec.ForProvider.AllowedCIDRs = []meta.IPv4CIDR{"10.0.0.1/32"}

	notReady := test.MySQL("notready", test.DefaultProject, "nine-es34")

	secret := testSecret(myName, test.DefaultProject, myUser, myPass)

	_, notFoundCmd := testDatabaseCmd("doesnotexist", &cidr)
	_, notReadyCmd := testDatabaseCmd("notready", &cidr)
	alreadyCap, alreadyPresentCmd := testDatabaseCmd(myName, &[]meta.IPv4CIDR{"10.0.0.1/32"})
	_, newCidrCmd := testDatabaseCmdConfirmed(myName, &cidr, true)
	credsCap, credsCmd := testDatabaseCmd(myName, &[]meta.IPv4CIDR{"10.0.0.1/32"})

	tests := []struct {
		name        string
		cmd         mysqlCmd
		cap         *capturingCmd
		wantErr     bool
		errContains string
		wantUpdate  bool
		checkArgs   func(t *testing.T, args []string)
	}{
		{
			name:    "resource not found",
			cmd:     mysqlCmd{serviceCmd: notFoundCmd},
			wantErr: true,
		},
		{
			name:        "resource not ready",
			cmd:         mysqlCmd{serviceCmd: notReadyCmd},
			wantErr:     true,
			errContains: "not ready",
		},
		{
			name: "cidr already present skips update",
			cmd:  mysqlCmd{serviceCmd: alreadyPresentCmd},
			cap:  alreadyCap,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				if !strings.Contains(strings.Join(args, " "), myFQDN) {
					t.Errorf("expected FQDN %q in args %v", myFQDN, args)
				}
			},
		},
		{
			name:       "new cidr triggers update",
			cmd:        mysqlCmd{serviceCmd: newCidrCmd},
			wantUpdate: true,
		},
		{
			name: "credentials appear in args",
			cmd:  mysqlCmd{serviceCmd: credsCmd},
			cap:  credsCap,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				joined := strings.Join(args, " ")
				if !strings.Contains(joined, myUser) {
					t.Errorf("expected user %q in args %v", myUser, args)
				}
				if !strings.Contains(joined, myPass) {
					t.Errorf("expected password in args %v", args)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updateCalled := false
			apiClient := test.SetupClient(t,
				test.WithObjects(ready, notReady, secret),
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
				my := &storage.MySQL{}
				if err := apiClient.Get(t.Context(), api.ObjectName(ready), my); err != nil {
					t.Fatalf("getting mysql: %v", err)
				}
				if !cidrsPresent(my.Spec.ForProvider.AllowedCIDRs, cidr) {
					t.Errorf("expected CIDR %v to be added, got %v", cidr, my.Spec.ForProvider.AllowedCIDRs)
				}
			}
		})
	}
}

