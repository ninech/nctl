package update

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const credentialsProject = "test-project"

func TestResetCredentialsAnnotates(t *testing.T) {
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
		"bucketuser":       bucketUserWithSecret(name, credentialsProject, location),
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			existing := connectionSecret(mg)
			apiClient := credentialsClient(t, nil, mg, existing)
			out := &bytes.Buffer{}

			is.NoError(runUpdate(t, apiClient, out, []string{kind, name, "--reset-credentials", "--force", "--wait=false"}))

			// the controller resets the credentials, nctl must not touch the secret.
			secret := &corev1.Secret{}
			is.NoError(apiClient.Get(t.Context(), api.ObjectName(existing), secret))
			is.Equal(existing.Data, secret.Data)

			stored, ok := mg.DeepCopyObject().(resource.Managed)
			is.True(ok)
			is.NoError(apiClient.Get(t.Context(), api.ObjectName(mg), stored))
			token := stored.GetAnnotations()[api.RotateCredentialsAnnotation]
			_, err := time.Parse(time.RFC3339, token)
			is.NoError(err, "expected the %s annotation to hold a time", api.RotateCredentialsAnnotation)

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
		// controller simulates the controller, if set.
		controller *fakeRotationController
		wantErr    string
		wantOutput string
		// wantAnnotated expects the reset to be requested.
		wantAnnotated  bool
		wantMemorySize string
		// wantHint expects the command to retrieve the new credentials to be suggested.
		wantHint bool
	}{
		"not requested": {
			args:           []string{"--memory-size=2G"},
			wantOutput:     "updated",
			wantMemorySize: "2G",
		},
		"confirmed": {
			args:          []string{"--reset-credentials", "--wait=false"},
			in:            "y\n",
			wantOutput:    "credentials reset",
			wantAnnotated: true,
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
			wantAnnotated:  true,
			wantMemorySize: "2G",
		},
		"done once the new credentials are applied": {
			args:          []string{"--reset-credentials", "--force", "--wait-timeout=5s"},
			controller:    &fakeRotationController{acknowledge: true, applyAfter: 5},
			wantOutput:    "new credentials of KeyValueStore \"test\" generated and applied",
			wantAnnotated: true,
			wantHint:      true,
		},
		"done if applied right away": {
			args:          []string{"--reset-credentials", "--force", "--wait-timeout=5s"},
			controller:    &fakeRotationController{acknowledge: true},
			wantOutput:    "new credentials of KeyValueStore \"test\" generated and applied",
			wantAnnotated: true,
			wantHint:      true,
		},
		"times out if the reset is not acknowledged": {
			args:          []string{"--reset-credentials", "--force", "--wait-timeout=500ms"},
			controller:    &fakeRotationController{},
			wantErr:       "the API did not acknowledge the credentials reset",
			wantAnnotated: true,
		},
		"times out while the new credentials are not applied": {
			args:          []string{"--reset-credentials", "--force", "--wait-timeout=500ms"},
			controller:    &fakeRotationController{acknowledge: true, applyAfter: -1},
			wantErr:       "the new credentials have not been applied yet",
			wantAnnotated: true,
		},
		"fails if superseded by another reset": {
			args:          []string{"--reset-credentials", "--force", "--wait-timeout=5s"},
			controller:    &fakeRotationController{acknowledge: true, applyAfter: -1, supersedeWith: "2099-01-01T00:00:00Z"},
			wantErr:       "superseded",
			wantAnnotated: true,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			kvs := testutil.KeyValueStore(name, credentialsProject, meta.LocationNineES34)
			existing := connectionSecret(kvs)
			apiClient := credentialsClient(t, tc.controller, kvs, existing)
			if tc.controller != nil {
				go tc.controller.run(t, apiClient, kvs.DeepCopy())
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

			updated := &storage.KeyValueStore{}
			is.NoError(apiClient.Get(t.Context(), api.ObjectName(kvs), updated))
			_, annotated := updated.GetAnnotations()[api.RotateCredentialsAnnotation]
			is.Equal(tc.wantAnnotated, annotated)
			if tc.wantMemorySize == "" {
				is.Nil(updated.Spec.ForProvider.MemorySize)
			} else {
				is.Equal(tc.wantMemorySize, updated.Spec.ForProvider.MemorySize.String())
			}

			// the controller resets the credentials, nctl must not touch the secret.
			secret := &corev1.Secret{}
			is.NoError(apiClient.Get(t.Context(), api.ObjectName(existing), secret))
			is.Equal(existing.Data, secret.Data)
		})
	}
}

// fakeRotationController simulates the credentials rotation of the controller.
// The typed resources do not know the status fields of the rotation yet,
// so the fake client cannot store them.
// It keeps them itself instead
// and adds them to the events of unstructured watches, see [fakeRotationController.watch].
type fakeRotationController struct {
	// acknowledge makes the controller reset the credentials
	// for the requested rotation.
	acknowledge bool
	// applyAfter is the number of ticks after the reset
	// until the new credentials are applied.
	// They are never applied if it is negative.
	applyAfter int
	// supersedeWith requests another rotation with this value after the reset.
	supersedeWith string

	mu        sync.Mutex
	last      string
	resetting bool
}

// run simulates the controller for mg.
// It updates mg on every tick
// to not depend on whether the wait already watches.
func (c *fakeRotationController) run(t *testing.T, client *api.Client, mg resource.Managed) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	ticksSinceReset := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		current, ok := mg.DeepCopyObject().(resource.Managed)
		if !ok {
			return
		}
		if err := client.Get(ctx, api.ObjectName(mg), current); err != nil {
			continue
		}
		annotations := current.GetAnnotations()
		requested := annotations[api.RotateCredentialsAnnotation]

		c.mu.Lock()
		switch {
		case !c.acknowledge || requested == "":
		case requested != c.last:
			c.last, c.resetting, ticksSinceReset = requested, true, 0
		default:
			ticksSinceReset++
		}
		if c.resetting && c.applyAfter >= 0 && ticksSinceReset >= c.applyAfter {
			c.resetting = false
		}
		supersede := c.supersedeWith != "" && c.last != "" && c.last != c.supersedeWith
		c.mu.Unlock()

		if supersede {
			annotations[api.RotateCredentialsAnnotation] = c.supersedeWith
			current.SetAnnotations(annotations)
		}
		// makes sure there is an event to observe the status.
		current.SetLabels(map[string]string{"tick": time.Now().Format("150405.000000")})
		_ = client.Update(ctx, current)
	}
}

// watch adds the status fields of the rotation to the events of unstructured watches.
// A typed watch would drop them.
func (c *fakeRotationController) watch(ctx context.Context, client runtimeclient.WithWatch, list runtimeclient.ObjectList, opts ...runtimeclient.ListOption) (watch.Interface, error) {
	w, err := client.Watch(ctx, list, opts...)
	if err != nil {
		return nil, err
	}
	if _, ok := list.(*unstructured.UnstructuredList); !ok {
		return w, nil
	}

	return watch.Filter(w, func(event watch.Event) (watch.Event, bool) {
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(event.Object)
		if err != nil {
			return event, true
		}
		c.mu.Lock()
		last, resetting := c.last, c.resetting
		c.mu.Unlock()
		if last == "" {
			return event, true
		}
		if err := unstructured.SetNestedField(content, last, "status", "atProvider", "lastCredentialsRotation"); err != nil {
			return event, true
		}
		if err := unstructured.SetNestedField(content, resetting, "status", "atProvider", "resettingCredentials"); err != nil {
			return event, true
		}
		event.Object = &unstructured.Unstructured{Object: content}
		return event, true
	}), nil
}

// credentialsClient returns a client with objects.
// If controller is set, unstructured watches report its status of the rotation.
func credentialsClient(t *testing.T, controller *fakeRotationController, objects ...runtimeclient.Object) *api.Client {
	t.Helper()

	opts := []testutil.ClientSetupOption{
		testutil.WithDefaultProject(credentialsProject),
		testutil.WithObjects(objects...),
	}
	if controller != nil {
		opts = append(opts, testutil.WithInterceptorFuncs(interceptor.Funcs{Watch: controller.watch}))
	}
	return testutil.SetupClient(t, opts...)
}

// connectionSecret returns the connection secret referenced by mg.
func connectionSecret(mg resource.Managed) *corev1.Secret {
	ref := mg.GetWriteConnectionSecretToReference()
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace},
		Data:       map[string][]byte{"password": []byte("old")},
	}
}

// bucketUserWithSecret returns a BucketUser with a connection secret reference,
// like nctl creates it.
func bucketUserWithSecret(name, project, location string) *storage.BucketUser {
	bu := bucketUser(name, project, location)
	bu.SetWriteConnectionSecretToReference(&runtimev1.SecretReference{Name: name, Namespace: project})
	return bu
}
