package auth

import (
	"context"
	stdErrors "errors"
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

func (s *SetProjectCmd) Run(ctx context.Context, client *api.Client) error {
	org, err := client.Organization()
	if err != nil {
		return err
	}

	err = client.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: org}, &management.Project{})

	if errors.IsNotFound(err) || errors.IsForbidden(err) {
		foundOrg, found, findErr := s.findProjectInOtherOrgs(ctx, client, org)
		if findErr != nil {
			return stdErrors.Join(err, findErr)
		}

		if found {
			if err := config.SetContextOrganization(client.KubeconfigPath, client.KubeconfigContext, foundOrg); err != nil {
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

	if err := config.SetContextProject(client.KubeconfigPath, client.KubeconfigContext, s.Name); err != nil {
		return err
	}

	fmt.Println(format.SuccessMessagef("📝", "set active Project to %s", s.Name))
	return nil
}
func (s *SetProjectCmd) findProjectInOtherOrgs(ctx context.Context, client *api.Client, currentOrg string) (string, bool, error) {
	userInfo, err := api.GetUserInfoFromToken(client.Token(ctx))
	if err != nil {
		return "", false, err
	}

	for _, targetOrg := range userInfo.Orgs {
		if targetOrg == currentOrg {
			continue
		}

		e := client.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: targetOrg}, &management.Project{})
		if e == nil {
			return targetOrg, true, nil
		}
		if !errors.IsNotFound(e) && !errors.IsForbidden(e) {
			return "", false, e
		}
	}

	return "", false, nil
}
