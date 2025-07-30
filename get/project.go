package get

import (
	"context"
	"sort"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
)

type projectCmd struct {
	resourceCmd
}

func (proj *projectCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	projectList, err := client.Projects(ctx, proj.Name)
	if err != nil {
		return err
	}

	if len(projectList) == 0 {
		return get.printEmptyMessage(management.ProjectKind, "")
	}

	// we sort alphabetically to have a deterministic output
	sort.Slice(
		projectList,
		func(i, j int) bool {
			return projectList[i].Name < projectList[j].Name
		},
	)

	switch get.Format {
	case full:
		return printProject(projectList, *get, true)
	case noHeader:
		return printProject(projectList, *get, false)
	case yamlOut:
		return format.PrettyPrintObjects(
			(&management.ProjectList{Items: projectList}).GetItems(),
			format.PrintOpts{
				Out:               get.writer,
				ExcludeAdditional: projectExcludes(),
			},
		)
	case jsonOut:
		return format.PrettyPrintObjects(
			(&management.ProjectList{Items: projectList}).GetItems(),
			format.PrintOpts{
				Out:               get.writer,
				ExcludeAdditional: projectExcludes(),
				Format:            format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: proj.Name != "",
				},
			},
		)
	}
	return nil
}

func printProject(projects []management.Project, get Cmd, header bool) error {
	// we don't want to include the PROJECT header as it doesn't make sense
	// for projects
	if header {
		get.AllProjects = false
		get.writeHeader("DISPLAY NAME")
	}

	for _, proj := range projects {
		displayName := proj.Spec.DisplayName
		if len(displayName) == 0 {
			displayName = util.NoneText
		}
		get.writeTabRow(proj.Name, displayName)
	}

	return get.tabWriter.Flush()
}

func projectExcludes() [][]string {
	return [][]string{
		{"spec"},
		{"status"},
	}
}
