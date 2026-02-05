package get

import (
	"context"
	"strconv"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/util/duration"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type configsCmd struct{}

func (cmd *configsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd)
}

func (cmd *configsCmd) list() client.ObjectList {
	return &apps.ProjectConfigList{}
}

func (cmd *configsCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	projectConfigList := list.(*apps.ProjectConfigList)
	if len(projectConfigList.Items) == 0 {
		return out.notFound(apps.ProjectConfigKind, client.Project)
	}

	switch out.Format {
	case full:
		return printProjectConfigs(projectConfigList.Items, out, true)
	case noHeader:
		return printProjectConfigs(projectConfigList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(projectConfigList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(projectConfigList.GetItems(), format.PrintOpts{Out: &out.Writer, Format: format.OutputFormatTypeJSON})
	}

	return nil
}

func printProjectConfigs(configs []apps.ProjectConfig, out *output, header bool) error {
	if header {
		out.writeHeader(
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

		out.writeTabRow(
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

	return out.tabWriter.Flush()
}
