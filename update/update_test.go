package update

import (
	"bytes"
	"context"
	"io"
	"maps"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	apps "github.com/ninech/apis/apps/v1alpha1"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/create"
	"github.com/ninech/nctl/internal/cli"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestUpdateWithoutChanges covers the two ways an update can end up not
// changing anything. An invocation without any update flag expresses no
// intent and is a usage error. An invocation whose flags describe the state
// the resource is already in is a no-op and has to succeed, so that repeating
// an update from a pipeline stays successful.
func TestUpdateWithoutChanges(t *testing.T) {
	t.Parallel()

	const (
		project     = "test-project"
		postgres    = "test-postgres"
		application = "test-application"
	)

	for name, tc := range map[string]struct {
		args []string
		// wantErr is the error the command is expected to return. An empty
		// value expects the command to succeed.
		wantErr string
		// wantOutput is expected in what the command printed.
		wantOutput string
		// wantMachineType is the machine type the postgres instance is
		// expected to have afterwards. An empty value skips the check.
		wantMachineType string
		// wantUpdate expects the resource to have been written to the API.
		wantUpdate bool
	}{
		"postgres without flags": {
			args:            []string{"postgres", postgres},
			wantErr:         "no flags provided",
			wantMachineType: infra.MachineTypeNineDBS.String(),
		},
		"postgres with a flag matching the current state": {
			args:            []string{"postgres", postgres, "--machine-type=" + infra.MachineTypeNineDBS.String()},
			wantOutput:      "no changes made",
			wantMachineType: infra.MachineTypeNineDBS.String(),
		},
		"postgres with a flag changing the current state": {
			args:            []string{"postgres", postgres, "--machine-type=" + infra.MachineTypeNineDBM.String()},
			wantOutput:      "updated",
			wantMachineType: infra.MachineTypeNineDBM.String(),
			wantUpdate:      true,
		},
		// The application command declares flags carrying a default, which
		// Kong reports as set even when the user does not pass them. They
		// must not be mistaken for an intent to change the application.
		"application without flags": {
			args:    []string{"application", application},
			wantErr: "no flags provided",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			existingPostgres := test.Postgres(postgres, project, "nine-es34")
			existingPostgres.Spec.ForProvider.MachineType = infra.MachineTypeNineDBS
			existingApplication := &apps.Application{
				ObjectMeta: metav1.ObjectMeta{Name: application, Namespace: project},
			}

			apiClient := test.SetupClient(
				t,
				test.WithDefaultProject(project),
				test.WithObjects(existingPostgres, existingApplication),
			)

			out := &bytes.Buffer{}
			err := runUpdate(t, apiClient, out, tc.args)

			if tc.wantErr != "" {
				var cliErr *cli.Error
				is.ErrorAs(err, &cliErr)
				// The wrapped error is compared instead of the rendered one,
				// which carries the display formatting.
				is.EqualError(cliErr.Err, tc.wantErr)
				is.Equal(cli.ExitUsageError, cliErr.ExitCode())
			} else {
				is.NoError(err)
			}

			if tc.wantOutput != "" {
				is.Contains(out.String(), tc.wantOutput)
			}

			if tc.wantMachineType != "" {
				updated := &storage.Postgres{}
				is.NoError(apiClient.Get(t.Context(), api.NamespacedName(postgres, project), updated))
				is.Equal(tc.wantMachineType, updated.Spec.ForProvider.MachineType.String())

				// The client annotates every resource it writes, which makes
				// the annotation a reliable probe for whether the update
				// reached the API at all.
				if tc.wantUpdate {
					is.Contains(updated.GetAnnotations(), cli.ManagedByAnnotation)
				} else {
					is.NotContains(updated.GetAnnotations(), cli.ManagedByAnnotation)
				}
			}
		})
	}
}

// runUpdate parses args and runs the resulting update command the way main
// does, so that the Kong hooks the commands rely on are applied.
func runUpdate(t *testing.T, client *api.Client, out io.Writer, args []string) error {
	t.Helper()

	var root struct {
		Postgres    postgresCmd    `cmd:"" name:"postgres"`
		Application applicationCmd `cmd:"" name:"application"`
	}

	applicationVars, err := create.ApplicationKongVars()
	if err != nil {
		t.Fatalf("application kong vars: %s", err)
	}
	vars := create.PostgresKongVars()
	maps.Copy(vars, applicationVars)

	parser := kong.Must(
		&root,
		kong.Name("nctl-test"),
		vars,
		kong.BindTo(t.Context(), (*context.Context)(nil)),
		kong.BindTo(out, (*io.Writer)(nil)),
	)

	kctx, err := parser.Parse(args)
	if err != nil {
		t.Fatalf("parse %s: %s", strings.Join(args, " "), err)
	}

	return kctx.Run(client)
}
