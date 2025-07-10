package auth

import (
	"context"
	"fmt"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

type SetProjectCmd struct {
	Name string `arg:"" help:"Name of the default project to be used." predictor:"project_name"`
}

func (s *SetProjectCmd) Run(ctx context.Context, client *api.Client) error {
	org, err := client.Organization()
	if err != nil {
		return err
	}

	// we get the project without using the result to be sure it exists and the
	// user has access.
	if err := client.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: org}, &management.Project{}); err != nil {
		if !errors.IsNotFound(err) && !errors.IsForbidden(err) {
			return err
		}
		if errors.IsNotFound(err) {
			format.PrintWarningf("did not find Project %s in your Organization %s.\n", s.Name, org)
		}
		if errors.IsForbidden(err) {
			format.PrintWarningf("you are not allowed to get the Project %s, you might not have access to all resources.\n", s.Name)
		}
	}

	if err := config.SetContextProject(client.KubeconfigPath, client.KubeconfigContext, s.Name); err != nil {
		return err
	}

	fmt.Println(format.SuccessMessagef("📝", "set active Project to %s", s.Name))
	return nil
}
