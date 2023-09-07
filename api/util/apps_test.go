package util

import (
	"reflect"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestEnvUpdate(t *testing.T) {
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
	up := &map[string]string{"old2": "val2"}
	del := &[]string{"old3"}
	new := UpdateEnvVars(old, up, del)
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

	assert.True(t, reflect.DeepEqual(new, expected), "env vars should be updated: %+v \n %+v", new, expected)
}
