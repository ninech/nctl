package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/ninech/nctl/internal/test"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestPostgresDatabaseCmd(t *testing.T) {
	t.Parallel()

	const (
		dbName = "mydb"
		dbFQDN = "mydb.example.com"
		dbUser = "mydb"
		dbPass = "dbsecret"
	)

	ready := test.PostgresDatabase(dbName, test.DefaultProject, "nine-es34")
	ready.Status.AtProvider.FQDN = dbFQDN
	ready.Status.AtProvider.Name = dbName

	notReady := test.PostgresDatabase("notready", test.DefaultProject, "nine-es34")

	secret := testSecret(dbName, test.DefaultProject, dbUser, dbPass)

	_, notFoundCmd := testDatabaseCmd("doesnotexist", nil)
	_, notReadyCmd := testDatabaseCmd("notready", nil)
	connectCap, connectCmd := testDatabaseCmd(dbName, nil)

	tests := []struct {
		name        string
		cmd         postgresDatabaseCmd
		cap         *capturingCmd
		wantErr     bool
		errContains string
		checkArgs   func(t *testing.T, args []string)
	}{
		{
			name:    "resource not found",
			cmd:     postgresDatabaseCmd{serviceCmd: notFoundCmd},
			wantErr: true,
		},
		{
			name:        "resource not ready",
			cmd:         postgresDatabaseCmd{serviceCmd: notReadyCmd},
			wantErr:     true,
			errContains: "not ready",
		},
		{
			name: "connects without cidr management",
			cmd:  postgresDatabaseCmd{serviceCmd: connectCmd},
			cap:  connectCap,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				joined := strings.Join(args, " ")
				if !strings.Contains(joined, dbFQDN) {
					t.Errorf("expected FQDN %q in args %v", dbFQDN, args)
				}
				if !strings.Contains(joined, dbName) {
					t.Errorf("expected dbname %q in args %v", dbName, args)
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
			if updateCalled {
				t.Error("Update must not be called for child database resources")
			}
			if !tc.wantErr && tc.checkArgs != nil {
				tc.checkArgs(t, tc.cap.args)
			}
		})
	}
}
