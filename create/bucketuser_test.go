package create

import (
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBucketUser(t *testing.T) {
	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		cmd             bucketUserCmd
		checkBucketUser func(t *testing.T, cmd bucketUserCmd, bu *storage.BucketUser)
	}{
		"create": {
			cmd: bucketUserCmd{
				resourceCmd: resourceCmd{Name: "create"},
				Location:    "nine-es34",
			},
			checkBucketUser: func(t *testing.T, cmd bucketUserCmd, bu *storage.BucketUser) {
				assert.Equal(t, "nine-es34", string(bu.Spec.ForProvider.Location))
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.cmd.Run(t.Context(), apiClient); err != nil {
				t.Fatal(err)
			}
			created := &storage.BucketUser{}
			if err := apiClient.Get(t.Context(), api.NamespacedName(tc.cmd.Name, apiClient.Project), created); err != nil {
				t.Fatal(err)
			}
			if tc.checkBucketUser != nil {
				tc.checkBucketUser(t, tc.cmd, created)
			}
		})
	}
}
