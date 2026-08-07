package update

import (
	"bytes"
	"context"
	"io"
	"maps"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
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

// TestUpdateWithoutChanges covers the invocations that end up not changing
// anything. None of them is an error: an update whose flags describe the state
// the resource is already in is a no-op, so that repeating an update from a
// pipeline stays successful, and an update without any flag has nothing to
// change either. All of them have to leave the resource untouched.
func TestUpdateWithoutChanges(t *testing.T) {
	t.Parallel()

	const (
		project          = "test-project"
		postgres         = "test-postgres"
		application      = "test-application"
		bucket           = "test-bucket"
		bucketWithPolicy = "test-bucket-with-policy"
	)

	for name, tc := range map[string]struct {
		args []string
		// wantOutput is expected in what the command printed.
		wantOutput string
		// wantMachineType is the machine type the postgres instance is
		// expected to have afterwards. An empty value skips the check.
		wantMachineType string
		// probe is read back from the API after the run to verify whether the
		// update reached it. It only needs to carry name and namespace.
		probe resource.Managed
		// wantUpdate expects the resource to have been written to the API.
		wantUpdate bool
	}{
		"postgres without flags": {
			args:            []string{"postgres", postgres},
			wantOutput:      "no changes made",
			wantMachineType: infra.MachineTypeNineDBS.String(),
			probe:           probeFor(&storage.Postgres{}, postgres, project),
		},
		"postgres with a flag matching the current state": {
			args:            []string{"postgres", postgres, "--machine-type=" + infra.MachineTypeNineDBS.String()},
			wantOutput:      "no changes made",
			wantMachineType: infra.MachineTypeNineDBS.String(),
			probe:           probeFor(&storage.Postgres{}, postgres, project),
		},
		"postgres with a flag changing the current state": {
			args:            []string{"postgres", postgres, "--machine-type=" + infra.MachineTypeNineDBM.String()},
			wantOutput:      "updated",
			wantMachineType: infra.MachineTypeNineDBM.String(),
			probe:           probeFor(&storage.Postgres{}, postgres, project),
			wantUpdate:      true,
		},
		// The application command declares flags carrying a default, which
		// Kong reports as set even when the user does not pass them. They
		// must not be mistaken for an intent to change the application.
		"application without flags": {
			args:       []string{"application", application},
			wantOutput: "no changes made",
			probe:      probeFor(&apps.Application{}, application, project),
		},
		// A flag carrying a default that is passed explicitly with its default
		// value asks for nothing, which is a no-op and not a usage error.
		"bucket with a defaulted flag passed as its default value": {
			args:       []string{"bucket", bucket, "--clear-lifecycle-policies=false"},
			wantOutput: "no changes made",
			probe:      probeFor(&storage.Bucket{}, bucket, project),
		},
		// The flag asks for an action here, but the bucket has no lifecycle
		// policies, so carrying it out changes nothing.
		"bucket with a defaulted flag that changes nothing": {
			args:       []string{"bucket", bucket, "--clear-lifecycle-policies"},
			wantOutput: "no changes made",
			probe:      probeFor(&storage.Bucket{}, bucket, project),
		},
		// The same flag on a bucket that does have a lifecycle policy has to
		// reach the API.
		"bucket with a defaulted flag that changes the resource": {
			args:       []string{"bucket", bucketWithPolicy, "--clear-lifecycle-policies"},
			wantOutput: "updated",
			probe:      probeFor(&storage.Bucket{}, bucketWithPolicy, project),
			wantUpdate: true,
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
			existingBucketWithPolicy := &storage.Bucket{
				ObjectMeta: metav1.ObjectMeta{Name: bucketWithPolicy, Namespace: project},
				Spec: storage.BucketSpec{
					ForProvider: storage.BucketParameters{
						LifecyclePolicies: []*storage.BucketLifecyclePolicy{
							{Prefix: "tmp/", ExpireAfterDays: 7, IsLive: true},
						},
					},
				},
			}

			apiClient := test.SetupClient(
				t,
				test.WithDefaultProject(project),
				test.WithObjects(
					existingPostgres,
					existingApplication,
					existingBucket,
					existingBucketWithPolicy,
				),
			)

			out := &bytes.Buffer{}
			is.NoError(runUpdate(t, apiClient, out, tc.args))

			if tc.wantOutput != "" {
				is.Contains(out.String(), tc.wantOutput)
			}

			is.NoError(apiClient.Get(t.Context(), api.ObjectName(tc.probe), tc.probe))

			if tc.wantMachineType != "" {
				pg, ok := tc.probe.(*storage.Postgres)
				is.True(ok)
				is.Equal(tc.wantMachineType, pg.Spec.ForProvider.MachineType.String())
			}

			// The client annotates every resource it writes, which makes the
			// annotation a reliable probe for whether the update reached the
			// API at all.
			if tc.wantUpdate {
				is.Contains(tc.probe.GetAnnotations(), cli.ManagedByAnnotation)
			} else {
				is.NotContains(tc.probe.GetAnnotations(), cli.ManagedByAnnotation)
			}
		})
	}
}

// probeFor names an empty resource so that it can be read back from the API
// after a run.
func probeFor[T resource.Managed](mg T, name, project string) T {
	mg.SetName(name)
	mg.SetNamespace(project)

	return mg
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
