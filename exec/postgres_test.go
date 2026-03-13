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

func TestPostgresCmd(t *testing.T) {
	t.Parallel()

	const (
		pgName   = "mypg"
		location = "nine-es34"
		fqdn     = "mypg.example.com"
		pgUser   = "admin"
		pgPass   = "secret"
	)

	cidr := []meta.IPv4CIDR{"203.0.113.5/32"}

	ready := test.Postgres(pgName, test.DefaultProject, location)
	ready.Status.AtProvider.FQDN = fqdn
	ready.Spec.ForProvider.AllowedCIDRs = []meta.IPv4CIDR{"10.0.0.1/32"}

	notReady := test.Postgres("notready", test.DefaultProject, location)

	secret := testSecret(pgName, test.DefaultProject, pgUser, pgPass)

	_, notFoundCmd := testDatabaseCmd("doesnotexist", &cidr)
	_, notReadyCmd := testDatabaseCmd("notready", &cidr)
	alreadyCap, alreadyPresentCmd := testDatabaseCmd(pgName, &[]meta.IPv4CIDR{"10.0.0.1/32"})
	newCidrCap, newCidrCmd := testDatabaseCmdConfirmed(pgName, &cidr, true)
	credsCap, credsCmd := testDatabaseCmd(pgName, &[]meta.IPv4CIDR{"10.0.0.1/32"})

	tests := []struct {
		name        string
		cmd         postgresCmd
		cap         *capturingCmd
		wantErr     bool
		errContains string
		wantUpdate  bool
		checkArgs   func(t *testing.T, args []string)
	}{
		{
			name:    "resource not found",
			cmd:     postgresCmd{serviceCmd: notFoundCmd},
			wantErr: true,
		},
		{
			name:        "resource not ready",
			cmd:         postgresCmd{serviceCmd: notReadyCmd},
			wantErr:     true,
			errContains: "not ready",
		},
		{
			name: "cidr already present skips update",
			cmd:  postgresCmd{serviceCmd: alreadyPresentCmd},
			cap:  alreadyCap,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				if !strings.Contains(strings.Join(args, " "), fqdn) {
					t.Errorf("expected FQDN %q in args %v", fqdn, args)
				}
			},
		},
		{
			name:       "new cidr triggers update",
			cmd:        postgresCmd{serviceCmd: newCidrCmd},
			cap:        newCidrCap,
			wantUpdate: true,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				if !strings.Contains(strings.Join(args, " "), fqdn) {
					t.Errorf("expected FQDN %q in args %v", fqdn, args)
				}
			},
		},
		{
			name: "credentials appear in args",
			cmd:  postgresCmd{serviceCmd: credsCmd},
			cap:  credsCap,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				joined := strings.Join(args, " ")
				if !strings.Contains(joined, pgUser) {
					t.Errorf("expected user %q in args %v", pgUser, args)
				}
				if !strings.Contains(joined, pgPass) {
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
			if !tc.wantUpdate && !tc.wantErr && updateCalled {
				t.Error("unexpected Update call when CIDR already present")
			}
			if !tc.wantErr && tc.checkArgs != nil {
				tc.checkArgs(t, tc.cap.args)
			}

			if tc.wantUpdate {
				pg := &storage.Postgres{}
				if err := apiClient.Get(t.Context(), api.ObjectName(ready), pg); err != nil {
					t.Fatalf("getting postgres: %v", err)
				}
				if !cidrsPresent(pg.Spec.ForProvider.AllowedCIDRs, cidr) {
					t.Errorf("expected CIDR %v to be added, got %v", cidr, pg.Spec.ForProvider.AllowedCIDRs)
				}
			}
		})
	}
}

