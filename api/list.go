package api

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"

	management "github.com/ninech/apis/management/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/conversion"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ListOpts struct {
	clientListOptions []runtimeclient.ListOption
	searchForName     string
	allProjects       bool
	allNamespaces     bool
	watch             bool
	watchFunc         WatchFunc
}

type ListOpt func(opts *ListOpts)

func MatchName(name string) ListOpt {
	return func(cmd *ListOpts) {
		if len(name) == 0 {
			cmd.searchForName = ""
			return
		}
		cmd.clientListOptions = append(cmd.clientListOptions, runtimeclient.MatchingFields{"metadata.name": name})
		cmd.searchForName = name
	}
}

func MatchLabel(k, v string) ListOpt {
	return func(cmd *ListOpts) {
		cmd.clientListOptions = append(cmd.clientListOptions, runtimeclient.MatchingLabels{k: v})
	}
}

func AllProjects() ListOpt {
	return func(cmd *ListOpts) {
		cmd.allProjects = true
	}
}

func AllNamespaces() ListOpt {
	return func(cmd *ListOpts) {
		cmd.allNamespaces = true
	}
}

func Watch(f WatchFunc) ListOpt {
	return func(opt *ListOpts) {
		opt.watch = true
		opt.watchFunc = f
	}
}

func (opts *ListOpts) namedResourceNotFound(project string, foundInProjects ...string) error {
	if opts.allProjects {
		return fmt.Errorf("resource %q was not found in any project", opts.searchForName)
	}
	errorMessage := fmt.Sprintf("resource %q was not found in project %s", opts.searchForName, project)
	if len(foundInProjects) > 0 {
		errorMessage = errorMessage + fmt.Sprintf(
			", but it was found in project(s): %s. "+
				"Maybe you want to use the '--project' flag to specify one of these projects?",
			strings.Join(foundInProjects, " ,"),
		)
	}
	return errors.New(errorMessage)
}

// ListObjects lists objects in the current client project with some
// ux-improvements like hinting when a resource has been found in a different
// project of the same organization.
func (c *Client) ListObjects(ctx context.Context, list runtimeclient.ObjectList, options ...ListOpt) error {
	opts := &ListOpts{}
	for _, opt := range options {
		opt(opts)
	}

	if opts.watch {
		return c.watch(ctx, list, options...)
	}

	if opts.allNamespaces {
		if err := c.List(ctx, list, opts.clientListOptions...); err != nil {
			return fmt.Errorf("error when listing across all namespaces: %w", err)
		}
		return nil
	}

	items, err := itemsFromObjectList(list)
	if err != nil {
		return err
	}

	if !opts.allProjects {
		// here a special logic applies. We are searching in the
		// current set project. If we are searching for a specific
		// named object and did not find it in the current set project,
		// we are searching in all projects for it. If we found it in
		// another project, we return an error saying that we found the
		// named object somewhere else.

		opts.clientListOptions = append(opts.clientListOptions, runtimeclient.InNamespace(c.Project))
		if err := c.List(ctx, list, opts.clientListOptions...); err != nil {
			return err
		}
		// if we did not search for a specific named object or we
		// actually found the object we were searching for in the
		// current project, we can stop here. If we were not able to
		// find it, we need to search in all projects for it.
		if opts.searchForName == "" || items.Len() > 0 {
			return nil
		}
	}
	// we want to search in all projects, so we need to get them first...
	projects, err := c.Projects(ctx, "")
	if err != nil {
		return fmt.Errorf("error when searching for projects: %w", err)
	}

	type ProjectItems struct {
		projectName string
		items       reflect.Value
	}

	projectsSize := len(projects)
	var wg sync.WaitGroup
	wg.Add(projectsSize)
	ch := make(chan ProjectItems, projectsSize)

	for _, proj := range projects {
		tempOpts := slices.Clone(opts.clientListOptions)
		go func() {
			defer wg.Done()
			// we ensured the list is a pointer type and that is has an
			// 'Items' field which is a slice above, so we don't need to do
			// this again here and instead use the reflect functions directly.
			tempList := reflect.New(reflect.TypeOf(list).Elem()).Interface().(runtimeclient.ObjectList)
			tempList.GetObjectKind().SetGroupVersionKind(list.GetObjectKind().GroupVersionKind())
			if err := c.List(ctx, tempList, append(tempOpts, runtimeclient.InNamespace(proj.Name))...); err != nil {
				c.writer.Warningf("error when searching in project %s: %s\n", proj.Name, err)
				return
			}
			tempListItems := reflect.ValueOf(tempList).Elem().FieldByName("Items")
			ch <- ProjectItems{projectName: proj.Name, items: tempListItems}
		}()
	}

	wg.Wait()
	close(ch)

	// Collect and sort by project name
	collected := make([]ProjectItems, 0, projectsSize)
	for pi := range ch {
		collected = append(collected, pi)
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].projectName < collected[j].projectName
	})

	for _, pi := range collected {
		for i := range pi.items.Len() {
			items.Set(reflect.Append(items, pi.items.Index(i)))
		}
	}

	// if the user did not search for a specific named resource we can already
	// quit as this case should not throw an error if no item could be
	// found
	if opts.searchForName == "" {
		return nil
	}

	// we can now be sure that the user searched for a named object in
	// either the current project or in all projects.
	if items.Len() == 0 {
		// we did not find the named object in any project. We return
		// an error here so that the command can be exited with a
		// non-zero code.
		return opts.namedResourceNotFound(c.Project)
	}
	// if the user searched in all projects for a specific resource and
	// something was found, we can already return with no error.
	if opts.allProjects {
		return nil
	}
	// we found the named object at least in one different project,
	// so we return a hint to the user to search in these projects
	var identifiedProjects []string
	for i := 0; i < items.Len(); i++ {
		// the "Items" field of a list type is a slice of types and not
		// a slice of pointer types (e.g. "[]corev1.Pod" and not
		// "[]*corev1.Pod"), but the clientruntime.Object interface is
		// implemented on pointer types (e.g. *corev1.Pod). So we need
		// to convert.
		if !items.Index(i).CanAddr() {
			// if the type of the "Items" slice is a pointer type
			// already, something is odd as this normally isn't the
			// case. We ignore the item in this case.
			continue
		}
		obj, isRuntimeClientObj := items.Index(i).Addr().Interface().(runtimeclient.Object)
		if !isRuntimeClientObj {
			// very unlikely case: the items of the list did not
			// implement runtimeclient.Object. As we can not get
			// the project of the object, we just ignore it.
			continue
		}
		identifiedProjects = append(identifiedProjects, obj.GetNamespace())
	}
	return opts.namedResourceNotFound(c.Project, identifiedProjects...)
}

// Projects returns either all existing Projects or only the specific project
// identified by the "onlyName" parameter
func (c *Client) Projects(ctx context.Context, onlyName string) ([]management.Project, error) {
	org, err := c.Organization()
	if err != nil {
		return nil, err
	}
	opts := []runtimeclient.ListOption{
		runtimeclient.InNamespace(org),
	}
	if onlyName != "" {
		opts = append(opts, runtimeclient.MatchingFields(
			map[string]string{"metadata.name": onlyName},
		))
	}

	projectList := &management.ProjectList{}
	if err := c.List(
		ctx,
		projectList,
		opts...,
	); err != nil {
		return nil, err
	}
	return projectList.Items, nil
}

func itemsFromObjectList(list runtimeclient.ObjectList) (reflect.Value, error) {
	// we need a bit of reflection code from the apimachinery package as the
	// ObjectList interface provides no way to get or set the list items
	// directly. We need to get a pointer to the items field of the list and
	// turn it into a reflect value so that we can change the items in case we
	// want to search in all projects.
	itemsPtr, err := meta.GetItemsPtr(list)
	if err != nil {
		return reflect.Value{}, err
	}
	return conversion.EnforcePtr(itemsPtr)
}
