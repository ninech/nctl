package get

import (
	"context"
	"io"
	"strconv"
	"text/tabwriter"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/util/duration"
)

type configsCmd struct {
	out io.Writer
}

func (cmd *configsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	projectConfigList := &apps.ProjectConfigList{}

	var opts []listOpt

	if !get.AllProjects {
		opts = []listOpt{matchName(client.Project)}
	}

	if err := get.list(ctx, client, projectConfigList, opts...); err != nil {
		return err
	}

	if len(projectConfigList.Items) == 0 {
		printEmptyMessage(cmd.out, apps.ProjectConfigKind, client.Project)
		return nil
	}

	switch get.Output {
	case full:
		return printProjectConfigs(projectConfigList.Items, get, defaultOut(cmd.out), true)
	case noHeader:
		return printProjectConfigs(projectConfigList.Items, get, defaultOut(cmd.out), false)
	case yamlOut:
		return format.PrettyPrintObjects(projectConfigList.GetItems(), format.PrintOpts{Out: defaultOut(cmd.out)})
	}

	return nil
}

func printProjectConfigs(configs []apps.ProjectConfig, get *Cmd, out io.Writer, header bool) error {
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(
			w,
			"NAME",
			"SIZE",
			"REPLICAS",
			"PORT",
			"ENVIRONMENT_VARIABLES",
			"AGE",
		)
	}

	for _, c := range configs {
		// Potential nil pointers. While these fields should never be empty
		// by the time a projectConfig is created, we should probably still check it.
		size := ""
		if c.Spec.ForProvider.Config.Size != nil {
			size = string(*c.Spec.ForProvider.Config.Size)
		}

		replicas := ""
		if c.Spec.ForProvider.Config.Replicas != nil {
			replicas = strconv.Itoa(int(*c.Spec.ForProvider.Config.Replicas))
		}

		port := ""
		if c.Spec.ForProvider.Config.Port != nil {
			port = strconv.Itoa(int(*c.Spec.ForProvider.Config.Port))
		}

		get.writeTabRow(
			w,
			c.ObjectMeta.Namespace,
			c.ObjectMeta.Name,
			size,
			replicas,
			port,
			util.EnvVarToString(c.Spec.ForProvider.Config.Env),
			duration.HumanDuration(time.Since(c.ObjectMeta.CreationTimestamp.Time)),
		)
	}

	return w.Flush()
}
