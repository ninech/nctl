package delete

import (
	"context"
	"fmt"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type applicationCmd struct {
	Name        string        `arg:"" help:"Name of the Application."`
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until Application is fully deleted."`
	WaitTimeout time.Duration `default:"10s" help:"Duration to wait for the deletion. Only relevant if wait is set."`
}

func (app *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, app.WaitTimeout)
	defer cancel()

	a := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: client.Namespace,
		},
	}

	d := newDeleter(a, apps.ApplicationKind, noCleanup)

	if err := d.deleteResource(ctx, client, app.WaitTimeout, app.Wait, app.Force); err != nil {
		return fmt.Errorf("error while deleting %s: %w", apps.ApplicationKind, err)
	}

	return nil
}
