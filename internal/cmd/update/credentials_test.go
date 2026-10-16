package update

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const credentialsProject = "test-project"

func TestResetCredentialsDeletesConnectionSecret(t *testing.T) {
	t.Parallel()

	const name = "test"
	location := string(meta.LocationNineES34)

	for kind, mg := range map[string]resource.Managed{
		"mysql":            testutil.MySQL(name, credentialsProject, location),
		"mysqldatabase":    testutil.MySQLDatabase(name, credentialsProject, location),
		"postgres":         testutil.Postgres(name, credentialsProject, location),
		"postgresdatabase": testutil.PostgresDatabase(name, credentialsProject, location),
		"keyvaluestore":    testutil.KeyValueStore(name, credentialsProject, meta.LocationNineES34),
		"opensearch":       testutil.OpenSearch(name, credentialsProject, meta.LocationNineES34),
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			apiClient := credentialsClient(t, mg, connectionSecret(mg))
			out := &bytes.Buffer{}

			is.NoError(runUpdate(t, apiClient, out, []string{kind, name, "--reset-credentials", "--force", "--wait=false"}))

			is.True(kerrors.IsNotFound(getConnectionSecret(t, apiClient, mg)))
			is.Contains(out.String(), "credentials reset")
			// resetting the credentials is not a spec change,
			// but it is not "no changes" either.
			is.NotContains(out.String(), "no changes made")
		})
	}
}

func TestResetCredentials(t *testing.T) {
	t.Parallel()

	const name = "test"

	for testName, tc := range map[string]struct {
		args []string
		in   string
		// withoutSecretRef removes the connection secret reference.
		withoutSecretRef bool
		// withoutSecret does not create the connection secret.
		withoutSecret bool
		// recreateSecret simulates the controller writing a new secret.
		recreateSecret bool
		wantErr        string
		wantOutput     string
		wantDeleted    bool
		wantMemorySize string
		// wantHint expects the command to retrieve the new credentials to be suggested.
		wantHint bool
	}{
		"not requested leaves the secret alone": {
			args:           []string{"--memory-size=2G"},
			wantOutput:     "updated",
			wantMemorySize: "2G",
		},
		"confirmed": {
			args:        []string{"--reset-credentials", "--wait=false"},
			in:          "y\n",
			wantOutput:  "credentials reset",
			wantDeleted: true,
		},
		"declined": {
			args:       []string{"--reset-credentials", "--wait=false"},
			in:         "n\n",
			wantOutput: "update canceled",
		},
		"declined does not apply other changes either": {
			args:       []string{"--reset-credentials", "--memory-size=2G", "--wait=false"},
			in:         "n\n",
			wantOutput: "update canceled",
		},
		"combined with a spec change": {
			args:           []string{"--reset-credentials", "--memory-size=2G", "--force", "--wait=false"},
			wantOutput:     "updated",
			wantDeleted:    true,
			wantMemorySize: "2G",
		},
		"no connection secret reference": {
			args:             []string{"--reset-credentials", "--force", "--wait=false"},
			withoutSecretRef: true,
			wantErr:          "has no connection secret",
		},
		"missing connection secret": {
			args:          []string{"--reset-credentials", "--force", "--wait=false"},
			withoutSecret: true,
			wantOutput:    "credentials reset",
			wantDeleted:   true,
		},
		"waits for the new secret": {
			args:           []string{"--reset-credentials", "--force", "--wait-timeout=5s"},
			recreateSecret: true,
			wantOutput:     "credentials of KeyValueStore \"test\" reset",
			wantHint:       true,
		},
		"times out without a new secret": {
			args:        []string{"--reset-credentials", "--force", "--wait-timeout=200ms"},
			wantErr:     "timeout waiting for credentials",
			wantDeleted: true,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			kvs := testutil.KeyValueStore(name, credentialsProject, meta.LocationNineES34)
			existing := connectionSecret(kvs)
			objects := []runtimeclient.Object{kvs}
			if !tc.withoutSecret {
				objects = append(objects, existing)
			}
			if tc.withoutSecretRef {
				kvs.SetWriteConnectionSecretToReference(nil)
			}
			apiClient := credentialsClient(t, objects...)

			if tc.recreateSecret {
				recreated := existing.DeepCopy()
				recreated.UID = "new"
				// the fake client stored the resource version on the existing secret,
				// which create refuses.
				recreated.ResourceVersion = ""
				go simulateController(t, apiClient, recreated)
			}

			out := &bytes.Buffer{}
			args := append([]string{"keyvaluestore", name}, tc.args...)
			err := runUpdateWithInput(t, apiClient, strings.NewReader(tc.in), out, args)
			if tc.wantErr != "" {
				is.ErrorContains(err, tc.wantErr)
			} else {
				is.NoError(err)
			}
			is.Contains(out.String(), tc.wantOutput)
			hint := "nctl get keyvaluestore test --project " + credentialsProject + " --print-connection-string"
			if tc.wantHint {
				is.Contains(out.String(), hint)
			} else {
				is.NotContains(out.String(), hint)
			}

			secret := &corev1.Secret{}
			getErr := apiClient.Get(t.Context(), api.ObjectName(existing), secret)
			switch {
			case tc.wantDeleted:
				is.True(kerrors.IsNotFound(getErr), "expected the connection secret to be deleted, got %v", getErr)
			case tc.recreateSecret:
				is.NoError(getErr)
				is.NotEqual(existing.UID, secret.UID)
			case !tc.withoutSecret:
				is.NoError(getErr)
				is.Equal(existing.UID, secret.UID)
			}

			updated := &storage.KeyValueStore{}
			is.NoError(apiClient.Get(t.Context(), api.ObjectName(kvs), updated))
			if tc.wantMemorySize == "" {
				is.Nil(updated.Spec.ForProvider.MemorySize)
			} else {
				is.Equal(tc.wantMemorySize, updated.Spec.ForProvider.MemorySize.String())
			}
		})
	}
}

// simulateController writes secret as soon as it is missing,
// like the controller does after the connection secret got deleted.
// It writes it repeatedly
// to not depend on whether the wait already watches.
func simulateController(t *testing.T, client *api.Client, secret *corev1.Secret) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := &corev1.Secret{}
			err := client.Get(ctx, api.ObjectName(secret), current)
			switch {
			case kerrors.IsNotFound(err):
				_ = client.Create(ctx, secret.DeepCopy())
			case err == nil:
				current.Data = map[string][]byte{"tick": []byte(time.Now().String())}
				_ = client.Update(ctx, current)
			}
		}
	}
}

func credentialsClient(t *testing.T, objects ...runtimeclient.Object) *api.Client {
	t.Helper()

	return testutil.SetupClient(t,
		testutil.WithDefaultProject(credentialsProject),
		testutil.WithObjects(objects...),
	)
}

// connectionSecret returns the connection secret referenced by mg.
func connectionSecret(mg resource.Managed) *corev1.Secret {
	ref := mg.GetWriteConnectionSecretToReference()
	return &corev1.Secret{
		// the fake client does not generate UIDs,
		// which the reset relies on.
		ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace, UID: "old"},
		Data:       map[string][]byte{"password": []byte("old")},
	}
}

func getConnectionSecret(t *testing.T, client *api.Client, mg resource.Managed) error {
	t.Helper()

	return client.Get(t.Context(), api.ObjectName(connectionSecret(mg)), &corev1.Secret{})
}
