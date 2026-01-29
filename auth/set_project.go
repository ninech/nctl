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
	Name string `arg:"" help:"Name of the default project to be used." completion-predictor:"project_name"`
}

func (s *SetProjectCmd) Run(ctx context.Context, apiClient *api.Client) error {
	org, err := apiClient.Organization()
	if err != nil {
		return err
	}

	// we get the project without using the result to be sure it exists and the user has access.
	err = apiClient.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: org}, &management.Project{})

	if errors.IsNotFound(err) || errors.IsForbidden(err) {
		foundOrg, findErr := orgFromProject(ctx, apiClient, s.Name)
		if findErr == nil {
			if err := config.SetContextOrganization(apiClient.KubeconfigPath, apiClient.KubeconfigContext, foundOrg); err != nil {
				return err
			}
			org = foundOrg
			err = nil
			format.PrintWarningf("Found project in org %s, switching...\n", org)
		}
	}

	if err != nil {
		if !errors.IsNotFound(err) && !errors.IsForbidden(err) {
			return err
		} else if errors.IsNotFound(err) {
			format.PrintWarningf("did not find Project %s in your Organization %s.\n", s.Name, org)
		} else if errors.IsForbidden(err) {
			format.PrintWarningf("you are not allowed to get the Project %s, you might not have access to all resources.\n", s.Name)
		}
	}

	if err := config.SetContextProject(apiClient.KubeconfigPath, apiClient.KubeconfigContext, s.Name); err != nil {
		return err
	}

	fmt.Println(format.SuccessMessagef("📝", "set active Project to %s", s.Name))
	return nil
}

func orgFromProject(ctx context.Context, apiClient *api.Client, projectName string) (string, error) {
	userInfo, err := api.GetUserInfoFromToken(apiClient.Token(ctx))
	if err != nil {
		return "", fmt.Errorf("could not get user info from token: %w", err)
	}

	for _, org := range userInfo.Orgs {
		proj := &management.Project{}
		if err := apiClient.Get(ctx, types.NamespacedName{Name: projectName, Namespace: org}, proj); err == nil {
			return org, nil
		}
	}

	return "", fmt.Errorf("could not find project %s in any available organization", projectName)
}
