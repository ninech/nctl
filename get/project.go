package get

import (
	"context"
	"io"
	"sort"
	"text/tabwriter"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type projectCmd struct {
	Name string `arg:"" help:"Name of the project to get. If omitted all projects will be listed." default:""`
	out  io.Writer
}

func (proj *projectCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	projectList, err := projects(ctx, client, proj.Name)
	if err != nil {
		return err
	}

	if len(projectList) == 0 {
		printEmptyMessage(proj.out, management.ProjectKind, "")
		return nil
	}

	// we sort alphabetically to have a deterministic output
	sort.Slice(
		projectList,
		func(i, j int) bool {
			return projectList[i].Name < projectList[j].Name
		},
	)

	switch get.Output {
	case full:
		return printProject(projectList, *get, defaultOut(proj.out), true)
	case noHeader:
		return printProject(projectList, *get, defaultOut(proj.out), false)
	case yamlOut:
		return format.PrettyPrintObjects(
			(&management.ProjectList{Items: projectList}).GetItems(),
			format.PrintOpts{
				Out:               proj.out,
				ExcludeAdditional: projectYamlExcludes(),
			},
		)
	}

	return nil
}

func printProject(projects []management.Project, get Cmd, out io.Writer, header bool) error {
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)

	// we don't want to include the PROJECT header as it doesn't make sense
	// for projects
	if header {
		get.AllProjects = false
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
