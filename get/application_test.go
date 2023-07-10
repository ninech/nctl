package get

import (
	"bytes"
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplication(t *testing.T) {
	app := apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: apps.ApplicationSpec{},
	}
	app2 := app
	app2.Name = app2.Name + "-2"

	get := &Cmd{
		Output: full,
	}

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&apps.Application{}, "metadata.name", func(o client.Object) []string {
			return []string{o.GetName()}
		}).
		WithObjects(&app, &app2).Build()
	apiClient := &api.Client{WithWatch: client, Project: "default"}
	ctx := context.Background()

	buf := &bytes.Buffer{}
	cmd := applicationsCmd{
		out:                  buf,
		BasicAuthCredentials: false,
	}

	if err := cmd.Run(ctx, apiClient, get); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 3, test.CountLines(buf.String()))
	buf.Reset()

	cmd.Name = app.Name
	if err := cmd.Run(ctx, apiClient, get); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, test.CountLines(buf.String()))
	buf.Reset()

	get.Output = noHeader
	if err := cmd.Run(ctx, apiClient, get); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, test.CountLines(buf.String()))
}

func TestApplicationCredentials(t *testing.T) {
	t.Parallel()

	const basicAuthNotFound = "no application with basic auth enabled found\n"
	ctx := context.Background()

	for name, testCase := range map[string]struct {
		resources     []client.Object
		name          string
		outputFormat  output
		project       string
		output        string
		errorExpected bool
	}{
		"no basic auth configured, all apps in project": {
			resources:    []client.Object{newBasicAuthApplication("dev", "dev", "")},
			outputFormat: full,
			project:      "dev",
			output:       basicAuthNotFound,
		},
		"no basic auth configured, specific app in project": {
			resources:    []client.Object{newBasicAuthApplication("dev", "dev", "")},
			outputFormat: full,
			project:      "dev",
			name:         "dev",
			output:       basicAuthNotFound,
		},
		"no basic auth configured, all apps in all projects": {
			resources:    []client.Object{newBasicAuthApplication("dev", "dev", "")},
			outputFormat: full,
			output:       basicAuthNotFound,
		},
		"missing basic auth secret leads to error": {
			resources:     []client.Object{newBasicAuthApplication("dev", "dev", "does-not-exist")},
			outputFormat:  full,
			name:          "dev",
			project:       "dev",
			output:        basicAuthNotFound,
			errorExpected: true,
		},
		"basic auth configured in one app and all apps in the project requested": {
			resources: []client.Object{
				newBasicAuthApplication("dev", "dev", "sample-basic-auth-secret"),
				newBasicAuthSecret(
					"sample-basic-auth-secret",
					"dev",
					util.BasicAuth{
						Username: "dev",
						Password: "sample",
					},
				),
			},
			outputFormat: full,
			project:      "dev",
			output: `NAME    USERNAME    PASSWORD
dev     dev         sample
`,
		},
		"basic auth configured in one app and all apps in the project requested, no header format": {
			resources: []client.Object{
				newBasicAuthApplication("dev", "dev", "sample-basic-auth-secret"),
				newBasicAuthSecret(
					"sample-basic-auth-secret",
					"dev",
					util.BasicAuth{
						Username: "dev",
						Password: "sample",
					},
				),
			},
			project:      "dev",
			outputFormat: noHeader,
			output:       "dev    dev    sample\n",
		},
		"basic auth configured in one app and all apps in the project requested, yaml format": {
			resources: []client.Object{
				newBasicAuthApplication("dev", "dev", "sample-basic-auth-secret"),
				newBasicAuthSecret(
					"sample-basic-auth-secret",
					"dev",
					util.BasicAuth{
						Username: "dev",
						Password: "sample",
					},
				),
			},
			project:      "dev",
			outputFormat: yamlOut,
			output:       "-\x1b[96m application\x1b[0m:\x1b[92m dev\x1b[0m\n\x1b[92m  \x1b[0m\x1b[96mproject\x1b[0m:\x1b[92m dev\x1b[0m\n\x1b[92m  \x1b[0m\x1b[96mbasicauth\x1b[0m:\x1b[96m\x1b[0m\n\x1b[96m    username\x1b[0m:\x1b[92m dev\x1b[0m\n\x1b[92m    \x1b[0m\x1b[96mpassword\x1b[0m:\x1b[92m sample\x1b[0m\n",
		},
		"multiple apps with basic auth configured and all apps in the project requested": {
			resources: []client.Object{
				newBasicAuthApplication("dev", "dev", "dev-basic-auth-secret"),
				newBasicAuthApplication("dev-second", "dev", "dev-second-basic-auth-secret"),
				newBasicAuthSecret(
					"dev-basic-auth-secret",
					"dev",
					util.BasicAuth{
						Username: "dev",
						Password: "sample",
					},
				),
				newBasicAuthSecret(
					"dev-second-basic-auth-secret",
					"dev",
					util.BasicAuth{
						Username: "dev-second",
						Password: "sample-second",
					},
				),
			},
			outputFormat: full,
			project:      "dev",
			output: `NAME          USERNAME      PASSWORD
dev           dev           sample
dev-second    dev-second    sample-second
`,
		},
		"multiple apps in different projects and all apps requested, yaml format": {
			resources: []client.Object{
				newBasicAuthApplication("dev", "dev", "dev-basic-auth-secret"),
				newBasicAuthApplication("prod", "prod", "prod-basic-auth-secret"),
				newBasicAuthSecret(
					"dev-basic-auth-secret",
					"dev",
					util.BasicAuth{
						Username: "dev",
						Password: "sample",
					},
				),
				newBasicAuthSecret(
					"prod-basic-auth-secret",
					"prod",
					util.BasicAuth{
						Username: "prod",
						Password: "secret",
					},
				),
			},
			outputFormat: yamlOut,
			output:       "-\x1b[96m application\x1b[0m:\x1b[92m dev\x1b[0m\n\x1b[92m  \x1b[0m\x1b[96mproject\x1b[0m:\x1b[92m dev\x1b[0m\n\x1b[92m  \x1b[0m\x1b[96mbasicauth\x1b[0m:\x1b[96m\x1b[0m\n\x1b[96m    username\x1b[0m:\x1b[92m dev\x1b[0m\n\x1b[92m    \x1b[0m\x1b[96mpassword\x1b[0m:\x1b[92m sample\x1b[0m\n\x1b[92m\x1b[0m-\x1b[96m application\x1b[0m:\x1b[92m prod\x1b[0m\n\x1b[92m  \x1b[0m\x1b[96mproject\x1b[0m:\x1b[92m prod\x1b[0m\n\x1b[92m  \x1b[0m\x1b[96mbasicauth\x1b[0m:\x1b[96m\x1b[0m\n\x1b[96m    username\x1b[0m:\x1b[92m prod\x1b[0m\n\x1b[92m    \x1b[0m\x1b[96mpassword\x1b[0m:\x1b[92m secret\x1b[0m\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			testCase := testCase
			get := &Cmd{
				Output:      testCase.outputFormat,
				AllProjects: testCase.project == "",
			}
			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&apps.Application{}, "metadata.name", func(o client.Object) []string {
					return []string{o.GetName()}
				}).
				WithObjects(testCase.resources...).
				Build()
			apiClient := &api.Client{WithWatch: client, Project: testCase.project}

			buf := &bytes.Buffer{}
			cmd := applicationsCmd{
				out:                  buf,
				Name:                 testCase.name,
				BasicAuthCredentials: true,
			}

			err = cmd.Run(ctx, apiClient, get)
			if testCase.errorExpected {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.output, buf.String())
		})
	}
}

func newBasicAuthApplication(name, project, secret string) *apps.Application {
	app := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       apps.ApplicationKind,
			APIVersion: apps.SchemeGroupVersion.String(),
		},
		Spec: apps.ApplicationSpec{
			ForProvider: apps.ApplicationParameters{
				Git: apps.ApplicationGitConfig{
					GitTarget: apps.GitTarget{
						URL:      "https://does-not-exist.example.com",
						Revision: "main",
					},
				},
			},
		},
	}
	if secret != "" {
		app.Status.AtProvider.BasicAuthSecret = &meta.LocalReference{Name: secret}
	}
	return app
}

func newBasicAuthSecret(name, project string, basicAuth util.BasicAuth) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		Data: map[string][]byte{
			util.BasicAuthUsernameKey: []byte(basicAuth.Username),
			util.BasicAuthPasswordKey: []byte(basicAuth.Password),
		},
	}
}
