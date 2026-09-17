package kubeconfig

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/cli"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/types"
)

// LoginCluster adds the KubernetesCluster name to the kubeconfig of client
// and switches the current context to it. If execPlugin is set, the OIDC
// exec plugin is run right away so the user is logged in and named in the
// output.
func LoginCluster(ctx context.Context, client *api.Client, w format.Writer, name types.NamespacedName, execPlugin bool) error {
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
		return fmt.Errorf("can not identify executable path of %s: %w", api.Name, err)
	}

	cfg, err := NewAPIConfig(
		apiEndpoint,
		issuerURL,
		command,
		cluster.Status.AtProvider.OIDCClientID,
		OverrideName(config.ContextName(cluster)),
		WithCACert(caCert),
	)
	if err != nil {
		return fmt.Errorf("unable to create kubeconfig: %w", err)
	}

	userInfo := &api.UserInfo{}

	if execPlugin {
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

	if err := Login(w, cfg, client.KubeconfigPath, userInfo.User, "", SwitchCurrentContext()); err != nil {
		return fmt.Errorf("error logging in to cluster %s: %w", name, err)
	}

	return nil
}
