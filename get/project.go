package get

import (
	"context"
	"io"
	"sort"
	"text/tabwriter"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/internal/format"
)

type projectCmd struct {
	Name string `arg:"" help:"Name of the project to get. If omitted all projects will be listed." default:""`
	out  io.Writer
}

func (proj *projectCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cfg, err := auth.ReadConfig(client.KubeconfigPath, client.KubeconfigContext)
	if err != nil {
		if auth.IsConfigNotFoundError(err) {
			return auth.ReloginNeeded(err)
		}
		return err
	}

	// projects can only be created in the main organization project so we
	// only need to search there
	client.Project = cfg.Organization
	get.AllProjects = false

	projectList := &management.ProjectList{}
	if err := get.list(ctx, client, projectList, matchName(proj.Name)); err != nil {
		return err
	}

	if len(projectList.Items) == 0 {
		printEmptyMessage(proj.out, management.ProjectKind, "")
		return nil
	}

	// we sort alphabetically to have a deterministic output
	sort.Slice(
		projectList.Items,
		func(i, j int) bool {
			return projectList.Items[i].Name < projectList.Items[j].Name
		},
	)

	switch get.Output {
	case full:
		return printProject(projectList.Items, get, defaultOut(proj.out), true)
	case noHeader:
		return printProject(projectList.Items, get, defaultOut(proj.out), false)
	case yamlOut:
		return format.PrettyPrintObjects(
			projectList.GetItems(),
			format.PrintOpts{
				Out:               proj.out,
				ExcludeAdditional: projectYamlExcludes(),
			},
		)
	}

	return nil
}

func printProject(projects []management.Project, get *Cmd, out io.Writer, header bool) error {
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME")
	}

	for _, proj := range projects {
		get.writeTabRow(w, "", proj.Name)
	}

	return w.Flush()
}

func projectYamlExcludes() [][]string {
	return [][]string{
		{"spec"},
		{"status"},
	}
}
