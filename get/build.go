package get

import (
	"context"
	"os"
	"text/tabwriter"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"k8s.io/apimachinery/pkg/util/duration"
)

type buildCmd struct {
	ApplicationName string `arg:"" help:"Name of the Application to get builds for. If omitted all in the namespace will be listed." default:""`
}

func (cmd *buildCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	buildList := &apps.BuildList{}

	opts := []listOpt{}
	if len(cmd.ApplicationName) != 0 {
		opts = append(opts, matchLabel(util.ApplicationNameLabel, cmd.ApplicationName))
	}
	if err := get.list(ctx, client, buildList, opts...); err != nil {
		return err
	}

	if len(buildList.Items) == 0 {
		printEmptyMessage(apps.BuildKind, client.Namespace)
		return nil
	}

	switch get.Output {
	case full:
		return printBuild(buildList.Items, get, true)
	case noHeader:
		return printBuild(buildList.Items, get, false)
	}

	return nil
}

func printBuild(builds []apps.Build, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "STATUS", "AGE")
	}

	for _, build := range builds {
		get.writeTabRow(w, build.Namespace, build.Name, string(build.Status.AtProvider.BuildStatus),
			duration.HumanDuration(time.Since(build.CreationTimestamp.Time)))
	}

	return w.Flush()
}
