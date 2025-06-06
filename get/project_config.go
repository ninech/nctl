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
	if err := get.list(ctx, client, projectConfigList); err != nil {
		return err
	}

	if len(projectConfigList.Items) == 0 {
		get.printEmptyMessage(cmd.out, apps.ProjectConfigKind, client.Project)
		return nil
	}

	switch get.Output {
	case full:
		return printProjectConfigs(projectConfigList.Items, get, defaultOut(cmd.out), true)
	case noHeader:
		return printProjectConfigs(projectConfigList.Items, get, defaultOut(cmd.out), false)
	case yamlOut:
		return format.PrettyPrintObjects(projectConfigList.GetItems(), format.PrintOpts{Out: defaultOut(cmd.out)})
	case jsonOut:
		return format.PrettyPrintObjects(projectConfigList.GetItems(), format.PrintOpts{Out: defaultOut(cmd.out), Format: format.OutputFormatTypeJSON})
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
			"BASIC_AUTH",
			"DEPLOY_JOB",
			"AGE",
		)
	}

	for _, c := range configs {
		// Potential nil pointers. While these fields should never be empty
		// by the time a projectConfig is created, we should probably still check it.

		replicas := ""
		if c.Spec.ForProvider.Config.Replicas != nil {
			replicas = strconv.Itoa(int(*c.Spec.ForProvider.Config.Replicas))
		}

		port := ""
		if c.Spec.ForProvider.Config.Port != nil {
			port = strconv.Itoa(int(*c.Spec.ForProvider.Config.Port))
		}

		basicAuth := false
		if c.Spec.ForProvider.Config.EnableBasicAuth != nil {
			basicAuth = *c.Spec.ForProvider.Config.EnableBasicAuth
		}

		deployJobName := util.NoneText
		if c.Spec.ForProvider.Config.DeployJob != nil {
			deployJobName = c.Spec.ForProvider.Config.DeployJob.Name
		}

		get.writeTabRow(
			w,
			c.Namespace,
			c.Name,
			string(c.Spec.ForProvider.Config.Size),
			replicas,
			port,
			util.EnvVarToString(c.Spec.ForProvider.Config.Env),
			strconv.FormatBool(basicAuth),
			deployJobName,
			duration.HumanDuration(time.Since(c.CreationTimestamp.Time)),
		)
	}

	return w.Flush()
}
