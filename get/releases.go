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

type releasesCmd struct {
	resourceCmd
	ApplicationName string `short:"a" help:"Name of the Application to get releases for. If omitted all applications in the project will be listed."`
}

func (cmd *releasesCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	opts := []api.ListOpt{api.MatchName(cmd.Name)}
	if len(cmd.ApplicationName) != 0 {
		opts = append(opts, api.MatchLabel(util.ApplicationNameLabel, cmd.ApplicationName))
	}

	return get.listPrint(ctx, client, cmd, opts...)
}

func (cmd *releasesCmd) list() client.ObjectList {
	return &apps.ReleaseList{}
}

func (cmd *releasesCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	releaseList := list.(*apps.ReleaseList)
	if len(releaseList.Items) == 0 {
		return out.printEmptyMessage(apps.ReleaseKind, client.Project)
	}

	util.OrderReleaseList(releaseList, true)

	switch out.Format {
	case full:
		return cmd.printReleases(releaseList.Items, out, true)
	case noHeader:
		return cmd.printReleases(releaseList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(releaseList.GetItems(), format.PrintOpts{Out: out.writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			releaseList.GetItems(),
			format.PrintOpts{
				Out:    out.writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func (cmd *releasesCmd) printReleases(releases []apps.Release, out *output, header bool) error {
	if header {
		out.writeHeader(
			"NAME",
			"BUILDNAME",
			"APPLICATION",
			"SIZE",
			"REPLICAS",
			"WORKERJOBS",
			"SCHEDULEDJOBS",
			"STATUS",
			"AGE",
		)
	}

	for _, r := range releases {
		// Potential nil pointers. While these fields should never be empty
		// by the time a release is created, we should probably still check it.

		replicas := ""
		cfg := r.Spec.ForProvider.Configuration.WithoutOrigin()
		if cfg.Replicas != nil {
			replicas = strconv.Itoa(int(*cfg.Replicas))
		}
		workerJobs := strconv.Itoa(len(cfg.WorkerJobs))
		scheduledJobs := strconv.Itoa(len(cfg.ScheduledJobs))

		out.writeTabRow(
			r.Namespace,
			r.Name,
			r.Spec.ForProvider.Build.Name,
			r.Labels[util.ApplicationNameLabel],
			string(cfg.Size),
			replicas,
			workerJobs,
			scheduledJobs,
			string(r.Status.AtProvider.ReleaseStatus),
			duration.HumanDuration(time.Since(r.CreationTimestamp.Time)),
		)
	}

	return out.tabWriter.Flush()
}
