package application

import (
	"reflect"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestEnvUpdate(t *testing.T) {
	t.Parallel()

	is := require.New(t)
	old := apps.EnvVars{
		{
			Name:  "old3",
			Value: "val3",
		},
		{
			Name:  "old2",
			Value: "toUpdate",
		},
		{
			Name:  "old1",
			Value: "val1",
		},
	}
	up := map[string]string{"old2": "val2"}
	del := []string{"old3"}
	new := UpdateEnvVars(old, up, nil, del)
	expected := apps.EnvVars{
		{
			Name:  "old1",
			Value: "val1",
		},
		{
			Name:  "old2",
			Value: "val2",
		},
	}

	is.True(reflect.DeepEqual(new, expected), "env vars should be updated: %+v \n %+v", new, expected)
}
