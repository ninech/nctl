package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/cli"
	"github.com/ninech/nctl/internal/format"

	"k8s.io/apimachinery/pkg/types"
)

type ClusterCmd struct {
	format.Writer `hidden:""`
	Name          string `arg:"" help:"Name of the cluster to authenticate with. Also accepts 'name/project' format."`
	ExecPlugin    bool   `help:"Automatically run exec plugin after writing the kubeconfig."`
}

func (a *ClusterCmd) Run(ctx context.Context, client *api.Client) error {
	name, err := clusterName(a.Name, client.Project)
	if err != nil {
		return err
	}

	cluster := &infrastructure.KubernetesCluster{}
	if err := client.Get(ctx, name, cluster); err != nil {
		return err
	}

	apiEndpoint, err := url.Parse(cluster.Status.AtProvider.APIEndpoint)
	if err != nil {
		return cli.ErrorWithContext(fmt.Errorf("invalid cluster API endpoint: %w", err)).
			WithExitCode(cli.ExitUsageError).
			WithContext("Endpoint", cluster.Status.AtProvider.APIEndpoint).
			WithSuggestions("The cluster API endpoint should be a valid URL")
	}

	issuerURL, err := url.Parse(cluster.Status.AtProvider.OIDCIssuerURL)
	if err != nil {
		return cli.ErrorWithContext(fmt.Errorf("invalid cluster OIDC issuer URL: %w", err)).
			WithExitCode(cli.ExitUsageError).
			WithContext("IssuerURL", cluster.Status.AtProvider.OIDCIssuerURL).
			WithSuggestions("The OIDC issuer URL should be a valid URL")
	}

	caCert, err := base64.StdEncoding.DecodeString(cluster.Status.AtProvider.APICACert)
	if err != nil {
		return fmt.Errorf("unable to decode API CA certificate: %w", err)
	}

	// not sure if this should ever happen but better than getting a panic
	if len(os.Args) == 0 {
		return fmt.Errorf("could not get command name from os.Args")
	}
	// we try to find out where the nctl binary is located
	command, err := os.Executable()
	if err != nil {
		return fmt.Errorf("can not identify executable path of %s: %w", cli.Name, err)
	}

	cfg, err := newAPIConfig(
		apiEndpoint,
		issuerURL,
		command,
		cluster.Status.AtProvider.OIDCClientID,
		overrideName(config.ContextName(cluster)),
		setCACert(caCert),
	)
	if err != nil {
		return fmt.Errorf("unable to create kubeconfig: %w", err)
	}

	userInfo := &api.UserInfo{}

	if a.ExecPlugin {
		authInfo, ok := cfg.AuthInfos[cfg.CurrentContext]
		if !ok {
			return fmt.Errorf("authInfo not found")
		}

		if authInfo == nil || authInfo.Exec == nil {
			return fmt.Errorf("no Exec found in authInfo")
		}

		token, err := api.GetTokenFromExecConfig(ctx, authInfo.Exec)
		if err != nil {
			return err
		}

		userInfo, err = api.GetUserInfoFromToken(token)
		if err != nil {
			return err
		}
	}

	if err := login(a.Writer, cfg, client.KubeconfigPath, userInfo.User, "", switchCurrentContext()); err != nil {
		return fmt.Errorf("error logging in to cluster %s: %w", name, err)
	}

	return nil
}

func clusterName(name, project string) (types.NamespacedName, error) {
	parts := strings.Split(name, "/")
	if len(parts) == 2 {
		name = parts[0]
		project = parts[1]
	}

	if project == "" {
		return types.NamespacedName{}, fmt.Errorf("project cannot be empty")
	}

	return types.NamespacedName{Name: name, Namespace: project}, nil
}
