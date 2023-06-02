package get

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/pkg/jsonmessage"
	"github.com/moby/term"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/util/duration"
)

type buildCmd struct {
	Name            string `arg:"" help:"Name of the Build to get. If omitted all in the namespace will be listed." default:""`
	ApplicationName string `short:"a" help:"Name of the Application to get builds for. If omitted all in the namespace will be listed."`
	PullImage       bool   `help:"Pull the image of the build. Uses the local docker socket at the env DOCKER_HOST if set."`
	out             io.Writer
}

func (cmd *buildCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	buildList := &apps.BuildList{}

	opts := []listOpt{matchName(cmd.Name)}
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

	if cmd.PullImage {
		if len(cmd.Name) == 0 {
			return fmt.Errorf("build name has to be specified for pulling an image")
		}

		return pullImage(ctx, client, &buildList.Items[0])
	}

	switch get.Output {
	case full:
		return printBuild(buildList.Items, get, defaultOut(cmd.out), true)
	case noHeader:
		return printBuild(buildList.Items, get, defaultOut(cmd.out), false)
	case yamlOut:
		return format.PrettyPrintObjects(buildList.GetItems(), format.PrintOpts{Out: defaultOut(cmd.out)})
	}

	return nil
}

func printBuild(builds []apps.Build, get *Cmd, out io.Writer, header bool) error {
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "APPLICATION", "STATUS", "AGE")
	}

	for _, build := range builds {
		get.writeTabRow(w, build.Namespace, build.Name,
			build.Labels[util.ApplicationNameLabel],
			string(build.Status.AtProvider.BuildStatus),
			duration.HumanDuration(time.Since(build.CreationTimestamp.Time)))
	}

	return w.Flush()
}

func pullImage(ctx context.Context, apiClient *api.Client, build *apps.Build) error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}

	registryAuth, err := registry.EncodeAuthConfig(registry.AuthConfig{
		// technically the username does not matter, it just needs to be set to something
		Username: "registry",
		Password: apiClient.Token,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Pulling image of build %s\n", build.Name)

	reader, err := cli.ImagePull(ctx, ImageRef(build.Spec.ForProvider.Image), types.ImagePullOptions{
		RegistryAuth: registryAuth,
	})
	if err != nil {
		return err
	}
	defer reader.Close()

	termFd, isTerm := term.GetFdInfo(os.Stderr)
	if err := jsonmessage.DisplayJSONMessagesStream(reader, os.Stderr, termFd, isTerm, nil); err != nil {
		return err
	}

	format.PrintSuccessf("💾", "Pulled image %s", imageName(build.Spec.ForProvider.Image))

	return nil
}

func ImageRef(image meta.Image) string {
	return imageName(image) + "@" + image.Digest
}

func imageName(image meta.Image) string {
	return path.Join(image.Registry, image.Repository)
}
