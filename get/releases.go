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

type releasesCmd struct {
	resourceCmd
	ApplicationName string `short:"a" help:"Name of the Application to get releases for. If omitted all applications in the project will be listed."`
	out             io.Writer
}

func (cmd *releasesCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	releaseList := &apps.ReleaseList{}
	opts := []api.ListOpt{api.MatchName(cmd.Name)}
	if len(cmd.ApplicationName) != 0 {
		opts = append(opts, api.MatchLabel(util.ApplicationNameLabel, cmd.ApplicationName))
	}

	if err := get.list(ctx, client, releaseList, opts...); err != nil {
		return err
	}

	if len(releaseList.Items) == 0 {
		get.printEmptyMessage(cmd.out, apps.ReleaseKind, client.Project)
		return nil
	}

	util.OrderReleaseList(releaseList, true)

	switch get.Output {
	case full:
		return cmd.printReleases(releaseList.Items, get, true)
	case noHeader:
		return cmd.printReleases(releaseList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(releaseList.GetItems(), format.PrintOpts{Out: defaultOut(cmd.out)})
	case jsonOut:
		return format.PrettyPrintObjects(releaseList.GetItems(), format.PrintOpts{Out: defaultOut(cmd.out), Format: format.OutputFormatTypeJSON})
	}

	return nil
}

func (cmd *releasesCmd) printReleases(releases []apps.Release, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(
			w,
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

		get.writeTabRow(
			w,
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

	return w.Flush()
}
