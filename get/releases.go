package get

import (
	"context"
	"io"
	"os"
	"sort"
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
	Name            string `arg:"" help:"Name of the Release to get. If omitted all in the namespace will be listed." default:""`
	ApplicationName string `help:"Name of the Application to get releases for. If omitted all in the namespace will be listed."`
	out             io.Writer
}

func (cmd *releasesCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	releaseList := &apps.ReleaseList{}

	opts := []listOpt{matchName(cmd.Name)}
	if len(cmd.ApplicationName) != 0 {
		opts = append(opts, matchLabel(util.ApplicationNameLabel, cmd.ApplicationName))
	}

	if err := get.list(ctx, client, releaseList, opts...); err != nil {
		return err
	}

	if len(releaseList.Items) == 0 {
		printEmptyMessage(cmd.out, apps.ReleaseKind, client.Namespace)
		return nil
	}

	orderReleaseList(releaseList)

	switch get.Output {
	case full:
		return printReleases(releaseList.Items, get, true)
	case noHeader:
		return printReleases(releaseList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(releaseList.GetItems(), format.PrintOpts{Out: defaultOut(cmd.out)})
	}

	return nil
}

func orderReleaseList(releaseList *apps.ReleaseList) {
	if len(releaseList.Items) <= 1 {
		return
	}

	sort.Slice(releaseList.Items, func(i, j int) bool {
		applicationNameI := releaseList.Items[i].ObjectMeta.Labels[util.ApplicationNameLabel]
		applicationNameJ := releaseList.Items[j].ObjectMeta.Labels[util.ApplicationNameLabel]

		if applicationNameI != applicationNameJ {
			return applicationNameI < applicationNameJ
		}

		return releaseList.Items[i].CreationTimestampNano < releaseList.Items[j].CreationTimestampNano
	})
}

func printReleases(releases []apps.Release, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(
			w,
			"NAME",
			"BUILDNAME",
			"APPLICATION",
			"SIZE",
			"REPLICAS",
			"STATUS",
			"AGE",
		)
	}

	for _, r := range releases {
		// Potential nil pointers. While these fields should never be empty
		// by the time a release is created, we should probably still check it.
		size := ""
		if r.Spec.ForProvider.Config.Size != nil {
			size = string(*r.Spec.ForProvider.Config.Size)
		}

		replicas := ""
		if r.Spec.ForProvider.Config.Replicas != nil {
			replicas = strconv.Itoa(int(*r.Spec.ForProvider.Config.Replicas))
		}

		get.writeTabRow(
			w,
			r.ObjectMeta.Namespace,
			r.ObjectMeta.Name,
			r.Spec.ForProvider.Build.Name,
			r.ObjectMeta.Labels[util.ApplicationNameLabel],
			size,
			replicas,
			string(r.Status.AtProvider.ReleaseStatus),
			duration.HumanDuration(time.Since(r.ObjectMeta.CreationTimestamp.Time)),
		)
	}

	return w.Flush()
}
