package api

import (
	"fmt"
	"strings"
)

// ExecArgs are the arguments of the exec plugins nctl writes to the kubeconfig,
// which run the [OIDCCmdName] or [ClientCredentialsCmdName] login of nctl.
type ExecArgs struct {
	// Command is [OIDCCmdName] or [ClientCredentialsCmdName].
	Command string
	// IssuerURL and UsePKCE configure the OIDC login.
	IssuerURL string
	UsePKCE   bool
	// Storage is where the OIDC login caches its tokens
	// and the client-credentials login stores its client secret.
	Storage  Storage
	ClientID string
	// TokenURL is the token endpoint of the client-credentials login.
	TokenURL string
	// ClientSecret is only set by kubeconfigs written before the client secret was stored,
	// newer ones leave it to [LookupClientSecret].
	ClientSecret string
}

// Args returns the arguments of the exec plugin.
// It panics if Command is neither [OIDCCmdName] nor [ClientCredentialsCmdName].
func (a ExecArgs) Args() []string {
	args := []string{AuthCmdName, a.Command}
	switch a.Command {
	case OIDCCmdName:
		args = append(args, IssuerURLArg+a.IssuerURL, ClientIDArg+a.ClientID)
		if a.UsePKCE {
			args = append(args, UsePKCEArg)
		}
	case ClientCredentialsCmdName:
		args = append(args, ClientIDArg+a.ClientID)
		if a.ClientSecret != "" {
			args = append(args, ClientSecretArg+a.ClientSecret)
		}
		args = append(args, TokenURLArg+a.TokenURL)
	default:
		panic(fmt.Sprintf("api: unknown exec plugin command %q", a.Command))
	}
	if a.Storage == StorageFile {
		args = append(args, InsecureStorageArg)
	}

	return args
}

// OIDCLogin returns the OIDC login the exec plugin runs.
func (a ExecArgs) OIDCLogin() OIDCLogin {
	return OIDCLogin{
		IssuerURL: a.IssuerURL,
		ClientID:  a.ClientID,
		UsePKCE:   a.UsePKCE,
		Storage:   a.Storage,
	}
}

// ParseExecArgs parses the arguments of an exec plugin written by [ExecArgs.Args].
// It ignores arguments it does not know.
func ParseExecArgs(args []string) (ExecArgs, error) {
	if len(args) < 2 {
		return ExecArgs{}, fmt.Errorf("exec plugin arguments %q are invalid, expected at least two", args)
	}

	a := ExecArgs{Command: args[1]}
	values := []struct {
		prefix string
		value  *string
	}{
		{IssuerURLArg, &a.IssuerURL},
		{ClientIDArg, &a.ClientID},
		{ClientSecretArg, &a.ClientSecret},
		{TokenURLArg, &a.TokenURL},
	}
	for _, arg := range args[2:] {
		switch arg {
		case UsePKCEArg:
			a.UsePKCE = true
		case InsecureStorageArg:
			a.Storage = StorageFile
		}
		for _, v := range values {
			if after, ok := strings.CutPrefix(arg, v.prefix); ok {
				*v.value = after
			}
		}
	}

	switch a.Command {
	case OIDCCmdName:
		if a.IssuerURL == "" || a.ClientID == "" {
			return ExecArgs{}, fmt.Errorf("exec plugin arguments do not include %s and %s", IssuerURLArg, ClientIDArg)
		}
	case ClientCredentialsCmdName:
		if a.ClientID == "" || a.TokenURL == "" {
			return ExecArgs{}, fmt.Errorf("exec plugin arguments do not include %s and %s", ClientIDArg, TokenURLArg)
		}
	default:
		return ExecArgs{}, fmt.Errorf("unknown exec plugin command %q", a.Command)
	}

	return a, nil
}
