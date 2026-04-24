package exec

import (
	"context"
	"encoding/base64"
	"os/exec"
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

	readyWithCA := test.MySQL(myName+"-ca", test.DefaultProject, "nine-es34")
	readyWithCA.Status.AtProvider.FQDN = myFQDN
	readyWithCA.Status.AtProvider.CACert = base64.StdEncoding.EncodeToString([]byte("fake-ca-cert"))
	readyWithCA.Spec.ForProvider.AllowedCIDRs = []meta.IPv4CIDR{"10.0.0.1/32"}

	notReady := test.MySQL("notready", test.DefaultProject, "nine-es34")

	secret := testSecret(myName, test.DefaultProject, myUser, myPass)
	secretWithCA := testSecret(myName+"-ca", test.DefaultProject, myUser, myPass)

	_, notFoundCmd := testDatabaseCmd("doesnotexist", &cidr)
	_, notReadyCmd := testDatabaseCmd("notready", &cidr)
	alreadyCap, alreadyPresentCmd := testDatabaseCmd(myName, &[]meta.IPv4CIDR{"10.0.0.1/32"})
	_, newCidrCmd := testDatabaseCmdConfirmed(myName, &cidr, true)
	credsCap, credsCmd := testDatabaseCmd(myName, &[]meta.IPv4CIDR{"10.0.0.1/32"})
	dbCap, dbCmd := testDatabaseCmd(myName, &[]meta.IPv4CIDR{"10.0.0.1/32"})
	sslCap, sslCmd := testDatabaseCmd(myName+"-ca", &[]meta.IPv4CIDR{"10.0.0.1/32"})

	tests := []struct {
		name        string
		cmd         mysqlCmd
		cap         *capturingCmd
		objects     []runtimeclient.Object
		wantErr     bool
		errContains string
		wantUpdate  bool
		check       func(t *testing.T, cmd *exec.Cmd)
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
			check: func(t *testing.T, cmd *exec.Cmd) {
				t.Helper()
				if !strings.Contains(strings.Join(cmd.Args, " "), myFQDN) {
					t.Errorf("expected FQDN %q in args %v", myFQDN, cmd.Args)
				}
			},
		},
		{
			name:       "new cidr triggers update",
			cmd:        mysqlCmd{serviceCmd: newCidrCmd},
			wantUpdate: true,
		},
		{
			name: "credentials passed securely",
			cmd:  mysqlCmd{serviceCmd: credsCmd},
			cap:  credsCap,
			check: func(t *testing.T, cmd *exec.Cmd) {
				t.Helper()
				argsStr := strings.Join(cmd.Args, " ")
				if strings.Contains(argsStr, myPass) {
					t.Errorf("password must not appear in args %v", cmd.Args)
				}
				if strings.Contains(argsStr, myUser) && !strings.Contains(argsStr, "--defaults-extra-file") {
					t.Errorf("user must not appear in plain args %v", cmd.Args)
				}
				if !strings.Contains(argsStr, "--defaults-extra-file=") {
					t.Errorf("expected --defaults-extra-file in args %v", cmd.Args)
				}
			},
		},
		{
			name: "custom database appears in args",
			cmd:  mysqlCmd{serviceCmd: dbCmd, Database: "mydb"},
			cap:  dbCap,
			check: func(t *testing.T, cmd *exec.Cmd) {
				t.Helper()
				if !strings.Contains(strings.Join(cmd.Args, " "), "mydb") {
					t.Errorf("expected database %q in args %v", "mydb", cmd.Args)
				}
			},
		},
		{
			name:    "ssl mode is VERIFY_CA when CA cert is present",
			cmd:     mysqlCmd{serviceCmd: sslCmd},
			cap:     sslCap,
			objects: []runtimeclient.Object{readyWithCA, secretWithCA},
			check: func(t *testing.T, cmd *exec.Cmd) {
				t.Helper()
				argsStr := strings.Join(cmd.Args, " ")
				if !strings.Contains(argsStr, "--ssl-mode=VERIFY_CA") {
					t.Errorf("expected --ssl-mode=VERIFY_CA in args %v", cmd.Args)
				}
				if strings.Contains(argsStr, "--ssl-mode=REQUIRED") {
					t.Errorf("unexpected --ssl-mode=REQUIRED when CA cert is present, args %v", cmd.Args)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			objs := []runtimeclient.Object{ready, notReady, secret}
			if len(tc.objects) > 0 {
				objs = tc.objects
			}
			updateCalled := false
			apiClient := test.SetupClient(t,
				test.WithObjects(objs...),
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
			if !tc.wantErr && tc.check != nil {
				tc.check(t, tc.cap.cmd)
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
