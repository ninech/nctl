package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecArgsRoundTrip(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args ExecArgs
		want []string
	}{
		"oidc": {
			args: ExecArgs{Command: OIDCCmdName, IssuerURL: "https://issuer", ClientID: "client", UsePKCE: true},
			want: []string{AuthCmdName, OIDCCmdName, IssuerURLArg + "https://issuer", ClientIDArg + "client", UsePKCEArg},
		},
		"oidc with file storage": {
			args: ExecArgs{Command: OIDCCmdName, IssuerURL: "https://issuer", ClientID: "client", Storage: StorageFile},
			want: []string{AuthCmdName, OIDCCmdName, IssuerURLArg + "https://issuer", ClientIDArg + "client", InsecureStorageArg},
		},
		"client credentials": {
			args: ExecArgs{Command: ClientCredentialsCmdName, ClientID: "id", TokenURL: "https://token"},
			want: []string{AuthCmdName, ClientCredentialsCmdName, ClientIDArg + "id", TokenURLArg + "https://token"},
		},
		"client credentials with file storage": {
			args: ExecArgs{Command: ClientCredentialsCmdName, ClientID: "id", TokenURL: "https://token", Storage: StorageFile},
			want: []string{AuthCmdName, ClientCredentialsCmdName, ClientIDArg + "id", TokenURLArg + "https://token", InsecureStorageArg},
		},
		"client credentials with secret": {
			args: ExecArgs{Command: ClientCredentialsCmdName, ClientID: "id", ClientSecret: "secret", TokenURL: "https://token"},
			want: []string{AuthCmdName, ClientCredentialsCmdName, ClientIDArg + "id", ClientSecretArg + "secret", TokenURLArg + "https://token"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			is.Equal(tt.want, tt.args.Args())
			parsed, err := ParseExecArgs(tt.want)
			is.NoError(err)
			is.Equal(tt.args, parsed)
		})
	}
}

func TestParseExecArgsErrors(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{
		"too short":                 {AuthCmdName},
		"unknown command":           {AuthCmdName, "unknown"},
		"oidc without issuer":       {AuthCmdName, OIDCCmdName, ClientIDArg + "client"},
		"client credentials no URL": {AuthCmdName, ClientCredentialsCmdName, ClientIDArg + "id"},
		"client credentials no ID":  {AuthCmdName, ClientCredentialsCmdName, TokenURLArg + "https://token"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseExecArgs(args)
			require.Error(t, err)
		})
	}
}

func TestExecArgsUnknownCommandPanics(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() { ExecArgs{Command: "unknown"}.Args() })
}
