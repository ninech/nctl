package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// all fields need to be pointers so we can detect if they have been set by
// the user.
type applicationCmd struct {
	Name     *string            `arg:"" help:"Name of the application."`
	Git      *gitConfig         `embed:"" prefix:"git-"`
	Size     *string            `help:"Size of the app."`
	Port     *int32             `help:"Port the app is listening on."`
	Replicas *int32             `help:"Amount of replicas of the running app."`
	Hosts    *[]string          `help:"Host names where the application can be accessed. If empty, the application will just be accessible on a generated host name on the deploio.app domain."`
	Env      *map[string]string `help:"Environment variables which are passed to the app at runtime."`
}

type gitConfig struct {
	URL           *string `help:"URL to the Git repository containing the application source. Both HTTPS and SSH formats are supported."`
	SubPath       *string `help:"SubPath is a path in the git repo which contains the application code. If not given, the root directory of the git repo will be used."`
	Revision      *string `help:"Revision defines the revision of the source to deploy the application to. This can be a commit, tag or branch."`
	Username      *string `help:"Username to use when authenticating to the git repository over HTTPS." env:"GIT_USERNAME"`
	Password      *string `help:"Password to use when authenticating to the git repository over HTTPS. In case of GitHub or GitLab, this can also be an access token." env:"GIT_PASSWORD"`
	SSHPrivateKey *string `help:"Private key in x509 format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY"`
}

func (cmd *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	// as name is a required arg this should not actually happen when called
	// through kong. But we still want to handle it in case this is called
	// directly.
	if cmd.Name == nil {
		return fmt.Errorf("name of the app has to be set")
	}

	app := &apps.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      *cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := newUpdater(client, app, apps.ApplicationKind, func(current resource.Managed) error {
		app, ok := current.(*apps.Application)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, apps.Application{})
		}

		cmd.applyUpdates(app)

		if cmd.Git != nil {
			auth := util.GitAuth{
				Username:      cmd.Git.Username,
				Password:      cmd.Git.Password,
				SSHPrivateKey: cmd.Git.SSHPrivateKey,
			}

			if auth.Enabled() {
				secret := auth.Secret(app.Name, client.Project)
				if err := client.Get(ctx, client.Name(secret.Name), secret); err != nil {
					return err
				}

				auth.UpdateSecret(secret)
				if err := client.Update(ctx, secret); err != nil {
					return err
				}
			}
		}

		return nil
	})

	return upd.Update(ctx)
}

func (cmd *applicationCmd) applyUpdates(app *apps.Application) {
	if cmd.Git != nil {
		if cmd.Git.URL != nil {
			app.Spec.ForProvider.Git.URL = *cmd.Git.URL
		}
		if cmd.Git.SubPath != nil {
			app.Spec.ForProvider.Git.SubPath = *cmd.Git.SubPath
		}
		if cmd.Git.Revision != nil {
			app.Spec.ForProvider.Git.Revision = *cmd.Git.Revision
		}
	}
	if cmd.Size != nil {
		newSize := apps.ApplicationSize(*cmd.Size)
		app.Spec.ForProvider.Config.Size = &newSize
	}
	if cmd.Port != nil {
		app.Spec.ForProvider.Config.Port = cmd.Port
	}
	if cmd.Replicas != nil {
		app.Spec.ForProvider.Config.Replicas = cmd.Replicas
	}
	if cmd.Hosts != nil {
		app.Spec.ForProvider.Hosts = *cmd.Hosts
	}
	if cmd.Env != nil {
		app.Spec.ForProvider.Config.Env = util.EnvVarsFromMap(*cmd.Env)
	}
}
