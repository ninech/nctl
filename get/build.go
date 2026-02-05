package get

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/docker/docker/api/types/image"
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
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dockerAPIVersion = "1.42"
)

type buildCmd struct {
	resourceCmd
	ApplicationName string `short:"a" help:"Name of the Application to get builds for. If omitted all in the project will be listed."`
	PullImage       bool   `help:"Pull the image of the build. Uses the local docker socket at the env DOCKER_HOST if set."`
}

func (cmd *buildCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	opts := []api.ListOpt{api.MatchName(cmd.Name)}
	if len(cmd.ApplicationName) != 0 {
		opts = append(opts, api.MatchLabel(util.ApplicationNameLabel, cmd.ApplicationName))
	}

	return get.listPrint(ctx, client, cmd, opts...)
}

func (cmd *buildCmd) list() runtimeclient.ObjectList {
	return &apps.BuildList{}
}

func (cmd *buildCmd) print(ctx context.Context, client *api.Client, list runtimeclient.ObjectList, out *output) error {
	buildList := list.(*apps.BuildList)
	if len(buildList.Items) == 0 {
		return out.printEmptyMessage(apps.BuildKind, client.Project)
	}

	if cmd.PullImage {
		if len(cmd.Name) == 0 {
			return fmt.Errorf("build name has to be specified for pulling an image")
		}

		return pullImage(ctx, client, &buildList.Items[0], out)
	}

	switch out.Format {
	case full:
		return printBuild(buildList.Items, out, true)
	case noHeader:
		return printBuild(buildList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(buildList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			buildList.GetItems(),
			format.PrintOpts{
				Out:    &out.Writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func printBuild(builds []apps.Build, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "APPLICATION", "STATUS", "AGE")
	}

	for _, build := range builds {
		out.writeTabRow(build.Namespace, build.Name,
			build.Labels[util.ApplicationNameLabel],
			string(build.Status.AtProvider.BuildStatus),
			duration.HumanDuration(time.Since(build.CreationTimestamp.Time)))
	}

	return out.tabWriter.Flush()
}

func pullImage(ctx context.Context, apiClient *api.Client, build *apps.Build, out *output) error {
	cli, err := client.NewClientWithOpts(client.WithVersion(dockerAPIVersion), client.FromEnv)
	if err != nil {
		return err
	}

	registryAuth, err := registry.EncodeAuthConfig(registry.AuthConfig{
		// technically the username does not matter, it just needs to be set to something
		Username: "registry",
		Password: apiClient.Token(ctx),
	})
	if err != nil {
		return err
	}

	out.Printf("Pulling image of build %s\n", build.Name)

	reader, err := cli.ImagePull(ctx, ImageRef(build.Spec.ForProvider.Image), image.PullOptions{
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

	if err := tagImage(ctx, cli, build); err != nil {
		return fmt.Errorf("unable to tag image: %w", err)
	}

	out.Successf("💾", "Pulled image %s", imageName(build.Spec.ForProvider.Image))

	return nil
}

func tagImage(ctx context.Context, cli *client.Client, build *apps.Build) error {
	// tag the pulled image with "latest" for ease of use
	if err := cli.ImageTag(ctx,
		ImageRef(build.Spec.ForProvider.Image),
		imageWithTag(build.Spec.ForProvider.Image, "latest"),
	); err != nil {
		return err
	}

	// tag the pulled image with the build name to tell versions apart
	return cli.ImageTag(ctx,
		ImageRef(build.Spec.ForProvider.Image),
		imageWithBuildTag(build),
	)
}

func ImageRef(image meta.Image) string {
	return imageName(image) + "@" + image.Digest
}

func imageName(image meta.Image) string {
	return path.Join(image.Registry, image.Repository)
}

func imageWithBuildTag(build *apps.Build) string {
	return imageWithTag(build.Spec.ForProvider.Image, build.Name)
}

func imageWithTag(image meta.Image, tag string) string {
	return imageName(image) + ":" + tag
}
