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
		bucket      = "test-bucket"
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
		// A flag which cannot change the resource expresses no intent to
		// update it either, no matter that the user passed it.
		"application with only a non-mutating flag": {
			args:    []string{"application", application, "--debug"},
			wantErr: "no flags provided",
		},
		// --clear-lifecycle-policies is a boolean flag whose zero value is
		// also its default. Kong cannot tell the two apart via Flag.Set, so
		// both of the following used to be reported as a usage error.
		"bucket with a clear flag that has nothing to clear": {
			args:       []string{"bucket", bucket, "--clear-lifecycle-policies"},
			wantOutput: "no changes made",
		},
		"bucket with an explicitly disabled clear flag": {
			args:       []string{"bucket", bucket, "--clear-lifecycle-policies=false"},
			wantOutput: "no changes made",
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
			existingBucket := &storage.Bucket{
				ObjectMeta: metav1.ObjectMeta{Name: bucket, Namespace: project},
			}

			apiClient := test.SetupClient(
				t,
				test.WithDefaultProject(project),
				test.WithObjects(existingPostgres, existingApplication, existingBucket),
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

// flagsProvidedEnv is the environment variable backing the env-tagged flag of
// [flagsProvidedCmd].
const flagsProvidedEnv = "NCTL_TEST_FLAGS_PROVIDED"

// flagsProvidedCmd covers the flag shapes the update commands use, so that
// [TestFlagsProvided] can exercise [ResourceCmd.AfterApply] against all of them
// instead of only against the ones a specific resource happens to declare.
type flagsProvidedCmd struct {
	ResourceCmd
	Size      *string `help:"A flag which is only set when the user passes it."`
	Toggle    bool    `help:"A flag whose zero value is indistinguishable from being unset."`
	Defaulted string  `help:"A flag carrying a default." default:"keep"`
	FromEnv   *string `help:"A flag which can be set from the environment." env:"NCTL_TEST_FLAGS_PROVIDED"`
	Negatable *bool   `negatable:"" help:"A flag which Kong parses under two names."`
	Debug     bool    `help:"A flag which cannot change the resource." nonmutating:""`
}

// TestFlagsProvided covers how [ResourceCmd.AfterApply] decides whether an
// invocation expressed an intent to update anything. The commands cannot answer
// that from their flag values, as Kong reports a flag as set no matter whether
// the value came from the command line, a default or the environment.
func TestFlagsProvided(t *testing.T) {
	// Not parallel, as the test cases set environment variables.
	for name, tc := range map[string]struct {
		args []string
		env  map[string]string
		want bool
	}{
		"no flags": {
			args: []string{"test", "resource"},
		},
		"flag the user passed": {
			args: []string{"test", "resource", "--size=mini"},
			want: true,
		},
		"boolean flag the user passed": {
			args: []string{"test", "resource", "--toggle"},
			want: true,
		},
		// A defaulted flag is set even when the user does not pass it, which is
		// why the default alone must not count as an intent to update.
		"defaulted flag left out": {
			args: []string{"test", "resource"},
		},
		// Passing a defaulted flag with the value it defaults to is
		// indistinguishable from the case above by flag value alone.
		"defaulted flag passed with its default value": {
			args: []string{"test", "resource", "--defaulted=keep"},
			want: true,
		},
		"defaulted flag passed with another value": {
			args: []string{"test", "resource", "--defaulted=change"},
			want: true,
		},
		"flag taken from the environment": {
			args: []string{"test", "resource"},
			env:  map[string]string{flagsProvidedEnv: "from-env"},
			want: true,
		},
		// Kong matches the negated name against the same flag, which has to be
		// recognised just like the positive one.
		"negated flag the user passed": {
			args: []string{"test", "resource", "--no-negatable"},
			want: true,
		},
		"flag which cannot change the resource": {
			args: []string{"test", "resource", "--debug"},
		},
		// Persistent flags belong to a parent node and never describe the
		// resource, so they express no intent to update it.
		"persistent flag of a parent command": {
			args: []string{"--project=test", "test", "resource"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			var root struct {
				Project string           `help:"A persistent flag of the root command."`
				Test    flagsProvidedCmd `cmd:"" name:"test"`
			}

			parser := kong.Must(
				&root,
				kong.Name("nctl-test"),
				kong.BindTo(io.Discard, (*io.Writer)(nil)),
			)

			_, err := parser.Parse(tc.args)
			require.NoError(t, err)
			require.Equal(t, tc.want, root.Test.flagsProvided)
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
		Bucket      bucketCmd      `cmd:"" name:"bucket"`
	}

	applicationVars, err := create.ApplicationKongVars()
	if err != nil {
		t.Fatalf("application kong vars: %s", err)
	}
	vars := create.PostgresKongVars()
	maps.Copy(vars, applicationVars)
	maps.Copy(vars, create.BucketKongVars())
	maps.Copy(vars, BucketKongVars())

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
