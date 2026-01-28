package auth

import (
	"context"
	"fmt"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/format"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ProjectOrganizationAnnotation = "management.nine.ch/organization"

type SetProjectCmd struct {
	Name string `arg:"" help:"Name of the default project to be used." completion-predictor:"project_name"`
}

func (s *SetProjectCmd) Run(ctx context.Context, apiClient *api.Client) error {
	org, err := apiClient.Organization()
	if err != nil {
		return err
	}

	err = apiClient.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: org}, &management.Project{})

	if errors.IsNotFound(err) || errors.IsForbidden(err) {
		proj, findErr := GetProjectFromNamespace(ctx, apiClient, s.Name)
		if findErr == nil && proj != nil {
			targetOrg := proj.Namespace
			if targetOrg != org {
				if err := config.SetContextOrganization(apiClient.KubeconfigPath, apiClient.KubeconfigContext, targetOrg); err != nil {
					return err
				}
				org = targetOrg
				err = nil
				fmt.Println(format.SuccessMessagef("🏢", "Found project in org %s, switching...", org))
			}
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

func GetOrgNameFromNamespace(ctx context.Context, c client.Reader, name string) (string, error) {
	ns := &corev1.Namespace{}
	if err := c.Get(ctx, types.NamespacedName{Name: name}, ns); err != nil {
		return "", fmt.Errorf("could not get project namespace: %w", err)
	}
	orgName, ok := ns.Annotations[ProjectOrganizationAnnotation]
	if ok {
		return orgName, nil
	}
	return name, nil
}

func GetProjectFromNamespace(ctx context.Context, c client.Reader, namespace string) (*management.Project, error) {
	org, err := GetOrgNameFromNamespace(ctx, c, namespace)
	if err != nil {
		return nil, fmt.Errorf("can not get organization for namespace %s: %w", namespace, err)
	}
	proj := &management.Project{}
	if err := c.Get(ctx, types.NamespacedName{Name: namespace, Namespace: org}, proj); err != nil {
		return nil, fmt.Errorf("can not get project for namespace %s: %w", namespace, err)
	}
	return proj, nil
}
