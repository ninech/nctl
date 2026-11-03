package auth

import (
	"strings"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/cli"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const defaultAPIURL = "https://nineapis.ch"

// KubeconfigClientSecretLogin returns the login that moves the client secret,
// which kubeconfigs written by earlier versions pass to the exec plugin exec as an argument,
// to the keyring of the operating system.
// ok is false if exec does not carry a client secret.
// apiURL and organization are those of the kubeconfig context; an empty organization is left out.
// The command shows a placeholder instead of the secret, so it can be printed.
func KubeconfigClientSecretLogin(exec *clientcmdapi.ExecConfig, apiURL, organization string) (login string, ok bool) {
	if exec == nil {
		return "", false
	}
	args, err := api.ParseExecArgs(exec.Args)
	if err != nil || args.Command != api.ClientCredentialsCmdName || args.ClientSecret == "" {
		return "", false
	}

	cmd := []string{cli.LoginCmd, "--api-client-id=" + args.ClientID, "--api-client-secret=<secret>"}
	if organization != "" {
		cmd = append(cmd, "--organization="+organization)
	}
	if apiURL != "" && strings.TrimSuffix(apiURL, "/") != defaultAPIURL {
		cmd = append(cmd, "--api-url="+apiURL)
	}
	if args.TokenURL != "" && args.TokenURL != defaultTokenURL {
		cmd = append(cmd, "--api-token-url="+args.TokenURL)
	}

	return strings.Join(cmd, " "), true
}
