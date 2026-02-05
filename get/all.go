package get

import (
	"context"
	"fmt"
	"sort"
	"strings"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	management "github.com/ninech/apis/management/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type allCmd struct {
	Kinds                []string `help:"Specify the kind of resources which should be listed."`
	IncludeNineResources bool     `help:"Show resources which are owned by Nine." default:"false"`
}

func (cmd *allCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	projectName := client.Project
	if get.AllProjects {
		projectName = ""
	}
	projectList, err := client.Projects(ctx, projectName)
	if err != nil {
		return err
	}

	items, warnings, err := cmd.getProjectContent(ctx, client, projectNames(projectList))
	if err != nil {
		return err
	}
	// we now print all warnings to stderr
	for _, w := range warnings {
		get.Warningf("%s", w)
	}

	if len(items) == 0 {
		return get.notFound("Resource", projectName)
	}

	switch get.Format {
	case full:
		return printItems(items, *get, true)
	case noHeader:
		return printItems(items, *get, false)
	case yamlOut:
		return format.PrettyPrintObjects(items, format.PrintOpts{Out: get.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			items,
			format.PrintOpts{Out: get.Writer, Format: format.OutputFormatTypeJSON},
		)
	}

	return nil
}

func projectNames(projects []management.Project) []string {
	result := make([]string, len(projects))
	for i, proj := range projects {
		result[i] = proj.Name
	}

	return result
}

func (cmd *allCmd) getProjectContent(
	ctx context.Context,
	client *api.Client,
	projNames []string,
) ([]*unstructured.Unstructured, []string, error) {
	var warnings []string
	var result []*unstructured.Unstructured
	listTypes, err := filteredListTypes(client.Scheme(), cmd.Kinds)
	if err != nil {
		return nil, nil, err
	}
	for _, project := range projNames {
		for _, listType := range listTypes {
			u := &unstructured.UnstructuredList{}
			u.SetGroupVersionKind(listType)
			// if we get any errors during the listing of certain
			// types we handle them as warnings to be able to
			// return as many resources as we can
			if err := client.List(ctx, u, runtimeclient.InNamespace(project)); err != nil {
				if !kerrors.IsForbidden(err) {
					warnings = append(warnings, err.Error())
				}
				continue
			}
			// we convert to a list of pointers so that we can
			// directly call DeepCopyObject() on them and also
			// filter nine owned resources if needed
			for _, item := range u.Items {
				if cmd.IncludeNineResources {
					result = append(result, &item)
					continue
				}
				if value, exists := item.GetLabels()[meta.NineOwnedLabelKey]; exists &&
					value == meta.NineOwnedLabelValue {
					continue
				}
				result = append(result, &item)
			}
		}
	}
	// we sort the items of the project to always have the same stable
	// output. We sort first by project, then by Kind and then by Name.
	sort.Slice(
		result,
		func(i, j int) bool {
			if result[i].GetNamespace() != result[j].GetNamespace() {
				return result[i].GetNamespace() < result[j].GetNamespace()
			}
			if result[i].GetKind() != result[j].GetKind() {
				return result[i].GetKind() < result[j].GetKind()
			}
			return result[i].GetName() < result[j].GetName()
		},
	)

	return result, warnings, nil
}

func printItems(items []*unstructured.Unstructured, get Cmd, header bool) error {
	// we always want to include the PROJECT (also in no header mode) as it
	// clearly indicates from which project the displayed resources are
	get.AllProjects = true

	if header {
		get.writeHeader("NAME", "KIND", "GROUP")
	}
	for _, item := range items {
		get.writeTabRow(
			item.GetNamespace(),
			item.GetName(),
			item.GroupVersionKind().Kind,
			item.GroupVersionKind().Group,
		)
	}

	return get.tabWriter.Flush()
}

func filteredListTypes(s *runtime.Scheme, kinds []string) ([]schema.GroupVersionKind, error) {
	result := []schema.GroupVersionKind{}
	lists := nineListTypes(s)
	if len(kinds) == 0 {
		return lists, nil
	}
OUTER:
	for _, kind := range kinds {
		for _, list := range lists {
			if !strings.EqualFold(kind+"list", list.GroupKind().Kind) {
				continue
			}
			result = append(result, list)
			continue OUTER
		}
		return []schema.GroupVersionKind{}, fmt.Errorf("kind %s does not seem to be part of any nine.ch API", kind)
	}
	return result, nil
}

func excludeListType(gvk schema.GroupVersionKind) bool {
	// ClusterData is a non-namespaced resource and used to allow
	// connecting to deplo.io application replicas.
	if strings.EqualFold(gvk.Kind, infrastructure.ClusterDataKind+"list") &&
		strings.EqualFold(gvk.Group, infrastructure.Group) {
		return true
	}
	return false
}

func nineListTypes(s *runtime.Scheme) []schema.GroupVersionKind {
	var lists []schema.GroupVersionKind
	for gvk := range s.AllKnownTypes() {
		if !strings.HasSuffix(strings.ToLower(gvk.Kind), "list") {
			continue
		}
		if excludeListType(gvk) {
			continue
		}
		if strings.HasSuffix(strings.ToLower(gvk.Group), "nine.ch") {
			lists = append(lists, gvk)
		}
	}
	// we sort the items to have a predicatable order of types in the output
	sort.Slice(
		lists,
		func(i, j int) bool {
			return lists[i].Kind < lists[j].Kind
		},
	)

	return lists
}
