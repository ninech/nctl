package auth

import (
	"context"
	"fmt"
	"slices"
	"strings"

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

	// Ensure the project exists. Try switching otherwise.
	if err := apiClient.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: org}, &management.Project{}); err != nil {
		if !errors.IsNotFound(err) && !errors.IsForbidden(err) {
			return fmt.Errorf("failed to set project %s: %w", s.Name, err)
		}

		format.PrintWarningf("Project does not exist in organization %s, checking other organizations...\n", org)
		if err := trySwitchOrg(ctx, apiClient, s.Name); err != nil {
			return fmt.Errorf("failed to switch organization: %w", err)
		}

		org, err = apiClient.Organization()
		if err != nil {
			return err
		}
	}

	if err := config.SetContextProject(apiClient.KubeconfigPath, apiClient.KubeconfigContext, s.Name); err != nil {
		return err
	}

	fmt.Println(format.SuccessMessagef("📝", "set active Project to %s in organization %s", s.Name, org))
	return nil
}

func trySwitchOrg(ctx context.Context, apiClient *api.Client, project string) error {
	org, err := orgFromProject(ctx, apiClient, project)
	if err != nil {
		return err
	}

	if err := config.SetContextOrganization(apiClient.KubeconfigPath, apiClient.KubeconfigContext, org); err != nil {
		return err
	}

	return nil
}

func orgFromProject(ctx context.Context, apiClient *api.Client, project string) (string, error) {
	userInfo, err := api.GetUserInfoFromToken(apiClient.Token(ctx))
	if err != nil {
		return "", fmt.Errorf("could not get user info from token: %w", err)
	}

	// If we can be sure that it is is the default project, there is no need to query the API.
	// This does not cover organizations that contain a "-".
	if !strings.Contains(project, "-") {
		if slices.Contains(userInfo.Orgs, project) {
			return project, nil
		}

		return "", fmt.Errorf("could not find project %s in any available organization", project)
	}

	for _, org := range userInfo.Orgs {
		proj := &management.Project{}
		err := apiClient.Get(ctx, types.NamespacedName{Name: project, Namespace: org}, proj)
		if errors.IsNotFound(err) || errors.IsForbidden(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("could not get project %s in org %s: %w", project, org, err)
		}

		return org, nil
	}

	return "", fmt.Errorf("could not find project %s in any available organization", project)
}
