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
	format.Writer
	Name string `arg:"" help:"Name of the default project to be used." completion-predictor:"project_name"`
}

func (s *SetProjectCmd) Run(ctx context.Context, client *api.Client) error {
	org, err := client.Organization()
	if err != nil {
		return err
	}

	// Ensure the project exists. Try switching otherwise.
	if err := client.Get(
		ctx,
		types.NamespacedName{Name: s.Name, Namespace: org},
		&management.Project{},
	); err != nil {
		if !errors.IsNotFound(err) && !errors.IsForbidden(err) {
			return fmt.Errorf("failed to set project %s: %w", s.Name, err)
		}

		s.Warningf(
			"Project does not exist in organization %s, checking other organizations...\n",
			org,
		)
		if err := trySwitchOrg(ctx, client, s.Name); err != nil {
			return fmt.Errorf("failed to switch organization: %w", err)
		}

		org, err = client.Organization()
		if err != nil {
			return err
		}
	}

	if err := config.SetContextProject(
		client.KubeconfigPath,
		client.KubeconfigContext,
		s.Name,
	); err != nil {
		return err
	}

	s.Successf("📝", "set active Project to %s in organization %s\n", s.Name, org)
	return nil
}

// trySwitchOrg attempts to find the organization containing the given project
// and switches the current context to that organization.
func trySwitchOrg(ctx context.Context, client *api.Client, project string) error {
	org, err := orgFromProject(ctx, client, project)
	if err != nil {
		return err
	}

	if err := config.SetContextOrganization(
		client.KubeconfigPath,
		client.KubeconfigContext,
		org,
	); err != nil {
		return err
	}

	return nil
}

// orgFromProject attempts to find the organization that contains the given project
// by checking all organizations the user is a member of.
func orgFromProject(ctx context.Context, client *api.Client, project string) (string, error) {
	userInfo, err := api.GetUserInfoFromToken(client.Token(ctx))
	if err != nil {
		return "", fmt.Errorf("could not get user info from token: %w", err)
	}

	// Avoid unnecessary API calls if some conditions are met.
	prefix, _, found := strings.Cut(project, "-")

	// If we can be sure that it is is the default project, there is no need to query the API.
	// This does not cover organizations that contain a "-".
	if !found {
		if slices.Contains(userInfo.Orgs, project) {
			return project, nil
		}

		return "", fmt.Errorf("could not find project %s in any available organization", project)
	}

	// Filter the organizations to check by only considering those that match the project prefix.
	// In most cases this will be a single organization.
	// But in cases where the organization name contains a "-", we need to check all organizations.
	orgs := func(yield func(string) bool) {
		for _, org := range userInfo.Orgs {
			if !strings.HasPrefix(org, prefix) {
				continue
			}

			if !yield(org) {
				return
			}
		}
	}

	for org := range orgs {
		proj := &management.Project{}
		err := client.Get(ctx, types.NamespacedName{Name: project, Namespace: org}, proj)
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
