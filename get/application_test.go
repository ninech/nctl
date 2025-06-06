package get

import (
	"bytes"
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestApplication(t *testing.T) {
	const otherProject = "my-pretty-other-project"
	app := apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: test.DefaultProject,
		},
		Spec: apps.ApplicationSpec{},
	}
	app2 := app
	app2.Name = app2.Name + "-2"

	app3 := app
	app3.Name = app.Name + "-3"
	app3.Namespace = otherProject

	get := &Cmd{
		Output: full,
	}

	apiClient, err := test.SetupClient(
		test.WithNameIndexFor(&apps.Application{}),
		test.WithProjectsFromResources(&app, &app2, &app3),
		test.WithObjects(&app, &app2, &app3),
		test.WithKubeconfig(t),
	)
	require.NoError(t, err)

	ctx := context.Background()
	buf := &bytes.Buffer{}
	cmd := applicationsCmd{
		out:                  buf,
		BasicAuthCredentials: false,
	}

	if err := cmd.Run(ctx, apiClient, get); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 3, test.CountLines(buf.String()), buf.String())
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

	// app3 is in a different project and we want to check if a hint gets
	// displayed along the error that it was not found
	cmd.Name = app3.Name
	buf.Reset()
	err = cmd.Run(ctx, apiClient, get)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), otherProject, err.Error())
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
			output: `PROJECT    NAME    USERNAME    PASSWORD
dev        dev     dev         sample
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
			output:       "dev    dev    dev    sample\n",
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
			output:       "application: dev\nbasicauth:\n  password: sample\n  username: dev\nproject: dev\n",
		},
		"basic auth configured in one app and all apps in the project requested, json format": {
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
			outputFormat: jsonOut,
			output:       `{"application":"dev","basicauth":{"password":"sample","username":"dev"},"project":"dev"}`,
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
			output: `PROJECT    NAME          USERNAME      PASSWORD
dev        dev           dev           sample
dev        dev-second    dev-second    sample-second
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
			output:       "application: dev\nbasicauth:\n  password: sample\n  username: dev\nproject: dev\n---\napplication: prod\nbasicauth:\n  password: secret\n  username: prod\nproject: prod\n",
		},
		"multiple apps in different projects and all apps requested, json format": {
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
			outputFormat: jsonOut,
			output:       `[{"application":"dev","basicauth":{"password":"sample","username":"dev"},"project":"dev"},{"application":"prod","basicauth":{"password":"secret","username":"prod"},"project":"prod"}]`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			testCase := testCase
			get := &Cmd{
				Output:      testCase.outputFormat,
				AllProjects: testCase.project == "",
			}

			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(testCase.resources...),
				test.WithObjects(testCase.resources...),
				test.WithKubeconfig(t),
				test.WithDefaultProject(testCase.project),
				test.WithNameIndexFor(&apps.Application{}),
			)
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			cmd := applicationsCmd{
				resourceCmd: resourceCmd{
					Name: testCase.name,
				},
				out:                  buf,
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

func TestApplicationDNS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	for name, testCase := range map[string]struct {
		apps          []client.Object
		name          string
		outputFormat  output
		project       string
		output        string
		errorExpected bool
	}{
		"no DNS set yet - full format": {
			apps: []client.Object{
				newApplicationWithDNS(
					"no-txt-record",
					"dev",
					txtRecordContent(""),
					"",
				),
			},
			outputFormat: full,
			project:      "dev",
			output: `PROJECT    NAME             TXT RECORD       DNS TARGET
dev        no-txt-record    <not set yet>    <not set yet>

Visit https://docs.nine.ch/a/myshbw3EY1 to see instructions on how to setup custom hosts
`,
		},
		"DNS set - one application - full format": {
			apps: []client.Object{
				newApplicationWithDNS(
					"sample",
					"dev",
					txtRecordContent("sample-dev-3ksdk23"),
					"sample.3ksdk23.deploio.app",
				),
			},
			outputFormat: full,
			project:      "dev",
			output: `PROJECT    NAME      TXT RECORD                                      DNS TARGET
dev        sample    deploio-site-verification=sample-dev-3ksdk23    sample.3ksdk23.deploio.app

Visit https://docs.nine.ch/a/myshbw3EY1 to see instructions on how to setup custom hosts
`,
		},
		"DNS set - one application - no header format": {
			apps: []client.Object{
				newApplicationWithDNS(
					"sample",
					"dev",
					txtRecordContent("sample-dev-3ksdk23"),
					"sample.3ksdk23.deploio.app",
				),
			},
			outputFormat: noHeader,
			project:      "dev",
			output: `dev    sample    deploio-site-verification=sample-dev-3ksdk23    sample.3ksdk23.deploio.app

Visit https://docs.nine.ch/a/myshbw3EY1 to see instructions on how to setup custom hosts
`,
		},
		"multiple applications in multiple projects - full format": {
			apps: []client.Object{
				newApplicationWithDNS(
					"sample",
					"dev",
					txtRecordContent("sample-dev-3ksdk23"),
					"sample.3ksdk23.deploio.app",
				),
				newApplicationWithDNS(
					"test",
					"test",
					txtRecordContent("test-test-4ksdk23"),
					"test.4ksdk23.deploio.app",
				),
			},
			outputFormat: full,
			output: `PROJECT    NAME      TXT RECORD                                      DNS TARGET
dev        sample    deploio-site-verification=sample-dev-3ksdk23    sample.3ksdk23.deploio.app
test       test      deploio-site-verification=test-test-4ksdk23     test.4ksdk23.deploio.app

Visit https://docs.nine.ch/a/myshbw3EY1 to see instructions on how to setup custom hosts
`,
		},
		"multiple applications in one project - yaml format": {
			apps: []client.Object{
				newApplicationWithDNS(
					"sample",
					"dev",
					txtRecordContent("sample-dev-3ksdk23"),
					"sample.3ksdk23.deploio.app",
				),
				newApplicationWithDNS(
					"test",
					"dev",
					txtRecordContent("test-dev-4ksdk23"),
					"test.4ksdk23.deploio.app",
				),
			},
			project:      "dev",
			outputFormat: yamlOut,
			output:       "application: sample\ncnameTarget: sample.3ksdk23.deploio.app\nproject: dev\ntxtRecord: deploio-site-verification=sample-dev-3ksdk23\n---\napplication: test\ncnameTarget: test.4ksdk23.deploio.app\nproject: dev\ntxtRecord: deploio-site-verification=test-dev-4ksdk23\n",
		},
		"multiple applications in one project - json format": {
			apps: []client.Object{
				newApplicationWithDNS(
					"sample",
					"dev",
					txtRecordContent("sample-dev-3ksdk23"),
					"sample.3ksdk23.deploio.app",
				),
				newApplicationWithDNS(
					"test",
					"dev",
					txtRecordContent("test-dev-4ksdk23"),
					"test.4ksdk23.deploio.app",
				),
			},
			project:      "dev",
			outputFormat: jsonOut,
			output:       `[{"application":"sample","project":"dev","txtRecord":"deploio-site-verification=sample-dev-3ksdk23","cnameTarget":"sample.3ksdk23.deploio.app"},{"application":"test","project":"dev","txtRecord":"deploio-site-verification=test-dev-4ksdk23","cnameTarget":"test.4ksdk23.deploio.app"}]`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			testCase := testCase
			get := &Cmd{
				Output:      testCase.outputFormat,
				AllProjects: testCase.project == "",
			}
			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(testCase.apps...),
				test.WithObjects(testCase.apps...),
				test.WithKubeconfig(t),
				test.WithDefaultProject(testCase.project),
			)
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			cmd := applicationsCmd{
				resourceCmd: resourceCmd{
					Name: testCase.name,
				},
				out:                  buf,
				BasicAuthCredentials: false,
				DNS:                  true,
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

func newApplication(name, project string) *apps.Application {
	return &apps.Application{
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
}

func newBasicAuthApplication(name, project, secret string) *apps.Application {
	app := newApplication(name, project)
	if secret != "" {
		app.Status.AtProvider.BasicAuthSecret = &meta.LocalReference{Name: secret}
	}
	return app
}

func newApplicationWithDNS(name, project, txtRecord, cnameRecord string) *apps.Application {
	app := newApplication(name, project)
	app.Status.AtProvider.TXTRecordContent = txtRecord
	app.Status.AtProvider.CNAMETarget = cnameRecord
	return app
}

func txtRecordContent(value string) string {
	if value == "" {
		return ""
	}
	return "deploio-site-verification=" + value
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
