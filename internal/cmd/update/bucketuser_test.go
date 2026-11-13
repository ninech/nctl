package update

import (
	"bytes"
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBucketUser(t *testing.T) {
	t.Parallel()

	const name = "user"

	for testName, tc := range map[string]struct {
		args          []string
		controller    *fakeRotationController
		wantErr       string
		wantOutput    string
		wantAnnotated bool
		wantHint      bool
	}{
		"no changes": {
			wantOutput: "no changes made",
		},
		"reset explicitly disabled": {
			args:       []string{"--reset-credentials=false"},
			wantOutput: "no changes made",
		},
		"reset without waiting": {
			args:          []string{"--reset-credentials=true", "--force", "--wait=false"},
			wantOutput:    "credentials reset of BucketUser \"user\" started",
			wantAnnotated: true,
		},
		"reset waits for the new credentials": {
			args:          []string{"--reset-credentials", "--force", "--wait-timeout=5s"},
			controller:    &fakeRotationController{acknowledge: true, applyAfter: 3},
			wantOutput:    "new credentials of BucketUser \"user\" generated and applied",
			wantAnnotated: true,
			wantHint:      true,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			bu := bucketUserWithSecret(name, credentialsProject, "nine-es34")
			existing := connectionSecret(bu)
			apiClient := credentialsClient(t, tc.controller, bu, existing)
			if tc.controller != nil {
				go tc.controller.run(t, apiClient, bu.DeepCopy())
			}

			out := &bytes.Buffer{}
			err := runUpdate(t, apiClient, out, append([]string{"bucketuser", name}, tc.args...))
			if tc.wantErr != "" {
				is.ErrorContains(err, tc.wantErr)
			} else {
				is.NoError(err)
			}
			is.Contains(out.String(), tc.wantOutput)
			hint := "nctl get bucketuser user --project " + credentialsProject + " --print-credentials"
			if tc.wantHint {
				is.Contains(out.String(), hint)
			} else {
				is.NotContains(out.String(), hint)
			}

			updated := &storage.BucketUser{}
			is.NoError(apiClient.Get(t.Context(), api.ObjectName(bu), updated))
			_, annotated := updated.GetAnnotations()[api.RotateCredentialsAnnotation]
			is.Equal(tc.wantAnnotated, annotated)
			// the deprecated field will be rejected by the API.
			is.Nil(updated.Spec.ForProvider.ResetCredentials) //nolint:staticcheck // SA1019 asserting the deprecated field stays unset.

			// the controller resets the credentials, nctl must not touch the secret.
			secret := &corev1.Secret{}
			is.NoError(apiClient.Get(t.Context(), api.ObjectName(existing), secret))
			is.Equal(existing.Data, secret.Data)
		})
	}
}

func bucketUser(name, project, location string) *storage.BucketUser {
	return &storage.BucketUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: storage.BucketUserSpec{
			ForProvider: storage.BucketUserParameters{
				Location: meta.LocationName(location),
			},
		},
	}
}
