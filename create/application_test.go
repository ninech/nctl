package create

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/grafana/loki/pkg/logcli/output"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

const (
	// dummySSHRSAPrivateKey is a dummy private RSA SSH key with some
	// whitespace around
	dummySSHRSAPrivateKey = `

-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAgv9MjQEnssfXn8OCcVMZQUS0iP9Cpo643RkS2ENvNlnXFTgy
mLX35RgkuHoAeQIlCFgYKA9bneJNRVDLcKQg3dElsMSdx6e2W879LChKrlhu904v
XYv09Txm6+MHd69agQEU8STWXrw39Fpdk8a36MMAEe+4SzSSoh0b/2wuwRLZmapS
gQF3HmqzxlfwupPUCbVtiWf6okJFO39TCI5vWD1bVUG5WqemVD+WY+AHjFrk41LM
+i+gwlnn392FUB+NrCYlx6dKXhGTr1IMX15l6JVtgDp3AvlvNGG6JrA/CLqoOf9f
Hv8pMLqPquVnDVNB/t3U0m7x/ZV2MUetklX6KwIDAQABAoIBAFjRZoLYPKVgEBe3
xLK3iBET12BnyjYJ4NewD3HoTvhH86fkgZG/F0QSmZsmxTlGtfsxV7eZqiGjdYbA
4B8QeWRMUUTIGr5rPR6Eem29J92MAjjVnxHLOhwohxP6y25fy3paVGun8V0sOrgH
qRjwDHPZ+ysuIQOEssMN/5SwMgcflXFgbLMjdNJxiP2TnJcjuEEHzDmXs7KAsch2
8dE6Wj+0W3FH1HRPxnlLfqALGxB+7I8CngXaRExbHYpWyFr+ke4PAGcLuHZ7eyZ0
86jqoo5ekyj0TTaflJVR851CQkRk9DWeFWEfQOeM5d17VWYqibHRs3r3jASMggus
61rmYRECgYEA4/blEB9CG6VRY0WMFljZrPtkr0zh0s01q3yUY95xizUOZknVLsk5
bOvf9Ovw5DL3YMMQ8a09MVGcUFIRq6KlNdoh87hYIipEii8lRB536r3yyNHGAGVo
BGon+iOZc/ma4U7pBewooUZGF5RgSlrSlGTwusgZRADUi7ncavU81tkCgYEAkxuL
Od6HaQkZP6OtUsf/pd+Y6xLjWm6Pf+xM6eu5PIyBsvWvcTBnit765Wy/VLDK1mzr
vNgSueIi6k41MjBJdCf91R6U3WulEV7xj9uPBeQbDMFDPoZPqEeyOqlb07D++Bk9
IJN6mVJWM/cOiQdJAhrXwqrk4vAsfeaiRKjx3qMCgYBPthE6pfNzv0bKM5NcbQ0Q
U4dNVNDR6TePEyzADxQc3Rx/3+lPRsVxtLjG54mAAeJGT28pUq5HBIZn/4p2PZUP
U4rzsc3/hFAbEYkyXIUJ7Als9w0JLmxEvunjqXcK+oiRqAoLLBy4592yeQuCdGeV
xAX5CebrxG6NvRu5uq7fYQKBgDd6j8tHTTIjqE4D4H3zx0o7RWSCPxP/1kacS3V8
3OMk6lUfqwa5BpOs/FpB5PZ/pj+v3EfgBU/tJNXQoOdIpqsT2friCapnylz+vYNP
fmTuXfU1fbK63JfOUj0lWehAPCg8/HyooffowXHfnq+2+6W7kdtsr92WTnE85b2X
KYCZAoGAZ8hdRurgNcmaBzQfRF/lYQVvlmBkCy00YmTeSrwLerWlFHsh7T8icBDT
k2dECAM99MLPJKkOwI/E0v1pAncQunLkWDJpWwb3egr+3Az+LE2TBTaDkP4kLgOw
sMVLxmbNrxvMSjJZlSiw3jYVOnXW2jZe+ceIN4LKRwW06ifnBpg=
-----END RSA PRIVATE KEY-----

`

	// dummySSHED25519PrivateKey is a dummy SSH private key in ed25519 format with
	// some whitespace around it
	dummySSHED25519PrivateKey = `

-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBSKbcOHZTe121IZz0EyMZMyvKPxRs8Rq1LTr+Uftr4zQAAAJDfP9T53z/U
+QAAAAtzc2gtZWQyNTUxOQAAACBSKbcOHZTe121IZz0EyMZMyvKPxRs8Rq1LTr+Uftr4zQ
AAAEDlLk0cOZ375YeCqvnfoTYl0pbFEGDaAAF4BwHqn6WqG1Iptw4dlN7XbUhnPQTIxkzK
8o/FGzxGrUtOv5R+2vjNAAAABm5vbmFtZQECAwQFBgc=
-----END OPENSSH PRIVATE KEY-----

`
)

func createTempKeyFile(content string) (string, error) {
	file, err := os.CreateTemp("", "temp-private-ssh-key.*.pem")
	if err != nil {
		return "", err
	}
	_, err = file.WriteString(content)
	if err != nil {
		return "", err
	}
	return file.Name(), nil
}
func TestApplication(t *testing.T) {
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	filenameRSAKey, err := createTempKeyFile(dummySSHRSAPrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filenameRSAKey)

	filenameED25519Key, err := createTempKeyFile(dummySSHED25519PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filenameED25519Key)

	cases := map[string]struct {
		cmd           applicationCmd
		checkApp      func(t *testing.T, cmd applicationCmd, app *apps.Application)
		errorExpected bool
	}{
		"without git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "superbug",
				},
				Wait:      false,
				Name:      "custom-name",
				Size:      pointer.String("mini"),
				Hosts:     []string{"custom.example.org", "custom2.example.org"},
				Port:      pointer.Int32(1337),
				Replicas:  pointer.Int32(42),
				BasicAuth: pointer.Bool(false),
				Env:       map[string]string{"hello": "world"},
				BuildEnv:  map[string]string{"BP_GO_TARGETS": "./cmd/web-server"},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Equal(t, cmd.Name, app.Name)
				assert.Equal(t, cmd.Git.URL, app.Spec.ForProvider.Git.URL)
				assert.Equal(t, cmd.Git.SubPath, app.Spec.ForProvider.Git.SubPath)
				assert.Equal(t, cmd.Git.Revision, app.Spec.ForProvider.Git.Revision)
				assert.Equal(t, cmd.Hosts, app.Spec.ForProvider.Hosts)
				assert.Equal(t, apps.ApplicationSize(*cmd.Size), app.Spec.ForProvider.Config.Size)
				assert.Equal(t, *cmd.Port, *app.Spec.ForProvider.Config.Port)
				assert.Equal(t, *cmd.Replicas, *app.Spec.ForProvider.Config.Replicas)
				assert.Equal(t, *cmd.BasicAuth, *app.Spec.ForProvider.Config.EnableBasicAuth)
				assert.Equal(t, util.EnvVarsFromMap(cmd.Env), app.Spec.ForProvider.Config.Env)
				assert.Equal(t, util.EnvVarsFromMap(cmd.BuildEnv), app.Spec.ForProvider.BuildEnv)
				assert.Nil(t, app.Spec.ForProvider.Git.Auth)
			},
		},
		"with basic auth": {
			cmd: applicationCmd{
				Wait:      false,
				Name:      "basic-auth",
				Size:      pointer.String("mini"),
				BasicAuth: pointer.Bool(true),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Equal(t, cmd.Name, app.Name)
				assert.Equal(t, apps.ApplicationSize(*cmd.Size), app.Spec.ForProvider.Config.Size)
				assert.Equal(t, *cmd.BasicAuth, *app.Spec.ForProvider.Config.EnableBasicAuth)
			},
		},
		"with user/pass git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					Username: pointer.String("deploy"),
					Password: pointer.String("hunter2"),
				},
				Wait: false,
				Name: "user-pass-auth",
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{Username: cmd.Git.Username, Password: cmd.Git.Password}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, *cmd.Git.Username, string(authSecret.Data[util.UsernameSecretKey]))
				assert.Equal(t, *cmd.Git.Password, string(authSecret.Data[util.PasswordSecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with ssh key git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:           "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKey: pointer.String(dummySSHRSAPrivateKey),
				},
				Wait: false,
				Name: "ssh-key-auth",
				Size: pointer.String("mini"),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{SSHPrivateKey: cmd.Git.SSHPrivateKey}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, strings.TrimSpace(*cmd.Git.SSHPrivateKey), string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with ssh ed25519 key git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:           "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKey: pointer.String(dummySSHED25519PrivateKey),
				},
				Wait: false,
				Name: "ssh-key-auth-ed25519",
				Size: pointer.String("mini"),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{SSHPrivateKey: cmd.Git.SSHPrivateKey}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, strings.TrimSpace(*cmd.Git.SSHPrivateKey), string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with ssh key git auth from file": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:                   "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKeyFromFile: pointer.String(filenameRSAKey),
				},
				Wait: false,
				Name: "ssh-key-auth-from-file",
				Size: pointer.String("mini"),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{SSHPrivateKey: pointer.String("notused")}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, strings.TrimSpace(dummySSHRSAPrivateKey), string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with ed25519 ssh key git auth from file": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:                   "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKeyFromFile: pointer.String(filenameED25519Key),
				},
				Wait: false,
				Name: "ssh-key-auth-from-file-ed25519",
				Size: pointer.String("mini"),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{SSHPrivateKey: pointer.String("notused")}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, strings.TrimSpace(dummySSHED25519PrivateKey), string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with non valid ssh key": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:           "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKey: pointer.String("not valid"),
				},
				Wait: false,
				Name: "ssh-key-auth-non-valid",
				Size: pointer.String("mini"),
			},
			errorExpected: true,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			app := tc.cmd.newApplication("default")

			err := tc.cmd.Run(ctx, apiClient)
			if tc.errorExpected {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}

			require.NoError(t, apiClient.Get(ctx, api.ObjectName(app), app))

			tc.checkApp(t, tc.cmd, app)
		})
	}
}

func TestApplicationWait(t *testing.T) {
	cmd := applicationCmd{
		Wait:        true,
		WaitTimeout: time.Second * 5,
		Name:        "some-name",
		BasicAuth:   pointer.Bool(true),
	}
	project := "default"

	build := &apps.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "any-name",
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: cmd.Name,
			},
		},
	}

	release := &apps.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "another-name",
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: cmd.Name,
			},
		},
	}

	// we are also creating a basic auth secret
	basicAuth := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-basic-auth",
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: cmd.Name,
			},
		},
		Data: map[string][]byte{
			util.BasicAuthUsernameKey: []byte("some-name"),
			util.BasicAuthPasswordKey: []byte("some-password"),
		},
	}

	// throw in a second build/release to ensure it can handle it
	build2 := *build
	build2.Name = build2.Name + "-1"
	release2 := *release
	release2.Name = release2.Name + "-1"

	apiClient, err := test.SetupClient(build, &build2, release, &release2, basicAuth)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// to test the wait we create a ticker that continously updates our
	// resources in a goroutine to simulate a controller doing the same
	ticker := time.NewTicker(100 * time.Millisecond)
	done := make(chan bool)
	errors := make(chan error, 1)

	go func() {
		for {
			select {
			case <-done:
				close(errors)
				return
			case <-ticker.C:
				app := &apps.Application{}
				if err := apiClient.Get(ctx, types.NamespacedName{Name: cmd.Name, Namespace: project}, app); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, app, runtimev1.ReconcileSuccess()); err != nil {
					errors <- err
				}

				app.Status.AtProvider.Hosts = []apps.VerificationStatus{{Name: "host.example.org"}}
				app.Status.AtProvider.CNAMETarget = "some.target.example.org"
				app.Status.AtProvider.BasicAuthSecret = &meta.LocalReference{Name: basicAuth.Name}
				if err := apiClient.Status().Update(ctx, app); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, build, runtimev1.Available()); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, &build2, runtimev1.Available()); err != nil {
					errors <- err
				}

				if err := apiClient.Get(ctx, api.ObjectName(build), build); err != nil {
					errors <- err
				}

				build.Status.AtProvider.BuildStatus = buildStatusRunning
				if err := apiClient.Status().Update(ctx, build); err != nil {
					errors <- err
				}

				build.Status.AtProvider.BuildStatus = buildStatusSuccess
				if err := apiClient.Status().Update(ctx, build); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, &release2, runtimev1.Available()); err != nil {
					errors <- err
				}

				release.Status.AtProvider.ReleaseStatus = releaseStatusAvailable
				if err := apiClient.Status().Update(ctx, release); err != nil {
					errors <- err
				}
			}
		}
	}()

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	ticker.Stop()
	done <- true

	for err := range errors {
		t.Fatal(err)
	}
}

func TestApplicationBuildFail(t *testing.T) {
	cmd := applicationCmd{
		Wait:        true,
		WaitTimeout: time.Second * 5,
		Name:        "some-name",
	}
	project := "default"

	build := &apps.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "any-name",
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: cmd.Name,
			},
		},
	}

	client, err := test.SetupClient(build)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	out, err := output.NewLogOutput(&buf, log.Mode("default"), &output.LogOutputOptions{
		NoLabels: true, ColoredOutput: false, Timezone: time.Local,
	})
	if err != nil {
		t.Fatal(err)
	}

	// fill our logs so we have more than errorLogLines
	logString := "does not compute!"
	buildLog := []string{}
	for i := 0; i < errorLogLines+30; i++ {
		buildLog = append(buildLog, logString)
	}
	client.Log = &log.Client{Client: log.NewFake(t, time.Now(), buildLog...), StdOut: out}

	ctx := context.Background()

	// to test the wait we create a ticker that continously updates our
	// resources in a goroutine to simulate a controller doing the same
	ticker := time.NewTicker(100 * time.Millisecond)
	done := make(chan bool)
	errors := make(chan error, 1)

	go func() {
		for {
			select {
			case <-done:
				close(errors)
				return
			case <-ticker.C:
				app := &apps.Application{ObjectMeta: metav1.ObjectMeta{
					Name:      cmd.Name,
					Namespace: project,
				}}

				if err := setResourceCondition(ctx, client, app, runtimev1.ReconcileSuccess()); err != nil {
					errors <- err
				}

				build.Status.AtProvider.BuildStatus = buildStatusError
				if err := client.Status().Update(ctx, build); err != nil {
					errors <- err
				}
			}
		}
	}()

	if err := cmd.Run(ctx, client); err == nil {
		t.Fatal("expected build error, got nil")
	}

	ticker.Stop()
	done <- true

	for err := range errors {
		t.Fatal(err)
	}

	assert.Contains(t, buf.String(), logString)
	assert.Equal(t, test.CountLines(buf.String()), errorLogLines)
}

func setResourceCondition(ctx context.Context, apiClient *api.Client, mg resource.Managed, condition runtimev1.Condition) error {
	if err := apiClient.Get(ctx, api.ObjectName(mg), mg); err != nil {
		return err
	}

	mg.SetConditions(condition)
	return apiClient.Status().Update(ctx, mg)
}
