package get

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/gobuffalo/flect"
	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/api/util"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/conversion"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Cmd struct {
	Output              output                `help:"Configures list output. ${enum}" short:"o" enum:"full,no-header,contexts,yaml,stats" default:"full"`
	AllProjects         bool                  `help:"apply the get over all projects." short:"A"`
	Clusters            clustersCmd           `cmd:"" group:"infrastructure.nine.ch" aliases:"cluster,vcluster" help:"Get Kubernetes Clusters."`
	APIServiceAccounts  apiServiceAccountsCmd `cmd:"" group:"iam.nine.ch" name:"apiserviceaccounts" aliases:"asa" help:"Get API Service Accounts."`
	Projects            projectCmd            `cmd:"" group:"management.nine.ch" name:"projects" aliases:"proj" help:"Get Projects."`
	Applications        applicationsCmd       `cmd:"" group:"deplo.io" name:"applications" aliases:"app,apps" help:"Get deplo.io Applications."`
	Builds              buildCmd              `cmd:"" group:"deplo.io" name:"builds" aliases:"build" help:"Get deplo.io Builds."`
	Releases            releasesCmd           `cmd:"" group:"deplo.io" name:"releases" aliases:"release" help:"Get deplo.io Releases."`
	Configs             configsCmd            `cmd:"" group:"deplo.io" name:"configs" aliases:"config" help:"Get deplo.io Project Configuration."`
	MySQL               mySQLCmd              `cmd:"" group:"storage.nine.ch" name:"mysql" help:"Get MySQL instances."`
	Postgres            postgresCmd           `cmd:"" group:"storage.nine.ch" name:"postgres" help:"Get PostgreSQL instances."`
	KeyValueStore       keyValueStoreCmd      `cmd:"" group:"storage.nine.ch" name:"keyvaluestore" aliases:"kvs" help:"Get KeyValueStore instances."`
	All                 allCmd                `cmd:"" name:"all" help:"Get project content"`
	CloudVirtualMachine cloudVMCmd            `cmd:"" group:"infrastructure.nine.ch" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Get a CloudVM."`
	opts                []runtimeclient.ListOption
	searchForName       string
}

type resourceCmd struct {
	Name string `arg:"" predictor:"resource_name" help:"Name of the resource to get. If omitted all in the project will be listed." default:""`
}

type output string

const (
	full     output = "full"
	noHeader output = "no-header"
	contexts output = "contexts"
	yamlOut  output = "yaml"
	stats    output = "stats"
)

type listOpt func(cmd *Cmd)

func matchName(name string) listOpt {
	return func(cmd *Cmd) {
		if len(name) == 0 {
			cmd.searchForName = ""
			return
		}
		cmd.opts = append(cmd.opts, runtimeclient.MatchingFields{"metadata.name": name})
		cmd.searchForName = name
	}
}

func matchLabel(k, v string) listOpt {
	return func(cmd *Cmd) {
		cmd.opts = append(cmd.opts, runtimeclient.MatchingLabels{k: v})
	}
}

func (cmd *Cmd) namedResourceNotFound(project string, foundInProjects ...string) error {
	if cmd.AllProjects {
		return fmt.Errorf("resource %q was not found in any project", cmd.searchForName)
	}
	errorMessage := fmt.Sprintf("resource %q was not found in project %s", cmd.searchForName, project)
	if len(foundInProjects) > 0 {
		errorMessage = errorMessage + fmt.Sprintf(
			", but it was found in project(s): %s. "+
				"Maybe you want to use the '--project' flag to specify one of these projects?",
			strings.Join(foundInProjects, " ,"),
		)
	}
	return errors.New(errorMessage)
}

func (cmd *Cmd) list(ctx context.Context, client *api.Client, list runtimeclient.ObjectList, opts ...listOpt) error {
	for _, opt := range opts {
		opt(cmd)
	}

	// we now need a bit of reflection code from the apimachinery package
	// as the ObjectList interface provides no way to get or set the list
	// items directly.

	// we need to get a pointer to the items field of the list and turn it
	// into a reflect value so that we can change the items in case we want
	// to search in all projects.
	itemsPtr, err := meta.GetItemsPtr(list)
	if err != nil {
		return err
	}
	items, err := conversion.EnforcePtr(itemsPtr)
	if err != nil {
		return err
	}

	if !cmd.AllProjects {
		// here a special logic applies. We are searching in the
		// current set project. If we are searching for a specific
		// named object and did not find it in the current set project,
		// we are searching in all projects for it. If we found it in
		// another project, we return an error saying that we found the
		// named object somewhere else.

		cmd.opts = append(cmd.opts, runtimeclient.InNamespace(client.Project))
		if err := client.List(ctx, list, cmd.opts...); err != nil {
			return err
		}
		// if we did not search for a specific named object or we
		// actually found the object we were searching for in the
		// current project, we can stop here. If we were not able to
		// find it, we need to search in all projects for it.
		if cmd.searchForName == "" || items.Len() > 0 {
			return nil
		}
	}
	// we want to search in all projects, so we need to get them first...
	projects, err := projects(ctx, client, "")
	if err != nil {
		return fmt.Errorf("error when searching for projects: %w", err)
	}

	for _, proj := range projects {
		tempOpts := slices.Clone(cmd.opts)
		// we ensured the list is a pointer type and that is has an
		// 'Items' field which is a slice above, so we don't need to do
		// this again here and instead use the reflect functions directly.
		tempList := reflect.New(reflect.TypeOf(list).Elem()).Interface().(runtimeclient.ObjectList)
		if err := client.List(ctx, tempList, append(tempOpts, runtimeclient.InNamespace(proj.Name))...); err != nil {
			return fmt.Errorf("error when searching in project %s: %w", proj.Name, err)
		}
		tempListItems := reflect.ValueOf(tempList).Elem().FieldByName("Items")
		for i := 0; i < tempListItems.Len(); i++ {
			items.Set(reflect.Append(items, tempListItems.Index(i)))
		}
	}

	// if the user did not search for a specific named resource we can already
	// quit as this case should not throw an error if no item could be
	// found
	if cmd.searchForName == "" {
		return nil
	}

	// we can now be sure that the user searched for a named object in
	// either the current project or in all projects.
	if items.Len() == 0 {
		// we did not find the named object in any project. We return
		// an error here so that the command can be exited with a
		// non-zero code.
		return cmd.namedResourceNotFound(client.Project)
	}
	// if the user searched in all projects for a specific resource and
	// something was found, we can already return with no error.
	if cmd.AllProjects {
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
	return cmd.namedResourceNotFound(client.Project, identifiedProjects...)
}

// writeHeader writes the header row, prepending the always shown project
func (cmd *Cmd) writeHeader(w io.Writer, headings ...string) {
	cmd.writeTabRow(w, "PROJECT", headings...)
}

// writeTabRow writes a row to w, prepending the passed project
func (cmd *Cmd) writeTabRow(w io.Writer, project string, row ...string) {
	if project == "" {
		// if the project is empty, the content should just be a "tab"
		// so that the structure will be contained
		project = "\t"
	}
	row = append([]string{project}, row...)

	switch length := len(row); length {
	case 0:
		break
	case 1:
		fmt.Fprintf(w, "%s\n", row[0])
	default:
		fmt.Fprintf(w, "%s", row[0])
		for _, r := range row[1:] {
			fmt.Fprintf(w, "\t%s", r)
		}
		fmt.Fprint(w, "\n")
	}
}

func (cmd *Cmd) printEmptyMessage(out io.Writer, kind, project string) {
	if cmd.AllProjects {
		fmt.Fprintf(defaultOut(out), "no %s found in any project\n", flect.Pluralize(kind))
		return
	}
	if project == "" {
		fmt.Fprintf(defaultOut(out), "no %s found\n", flect.Pluralize(kind))
		return
	}

	fmt.Fprintf(defaultOut(out), "no %s found in project %s\n", flect.Pluralize(kind), project)
}

func defaultOut(out io.Writer) io.Writer {
	if out == nil {
		return os.Stdout
	}
	return out
}

// projects returns either all existing projects or only the specific project
// identified by the "onlyName" parameter
func projects(ctx context.Context, client *api.Client, onlyName string) ([]management.Project, error) {
	cfg, err := config.ReadExtension(client.KubeconfigPath, client.KubeconfigContext)
	if err != nil {
		if config.IsExtensionNotFoundError(err) {
			return nil, util.ReloginNeeded(err)
		}
		return nil, err
	}
	opts := []runtimeclient.ListOption{
		runtimeclient.InNamespace(cfg.Organization),
	}
	if onlyName != "" {
		opts = append(opts, runtimeclient.MatchingFields(
			map[string]string{"metadata.name": onlyName},
		))
	}

	projectList := &management.ProjectList{}
	if err := client.List(
		ctx,
		projectList,
		opts...,
	); err != nil {
		return nil, err
	}
	return projectList.Items, nil
}

func getConnectionSecret(ctx context.Context, client *api.Client, key string, mg resource.Managed) (string, error) {
	secret, err := client.GetConnectionSecret(ctx, mg)
	if err != nil {
		return "", fmt.Errorf("unable to get connection secret: %w", err)
	}

	content, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %s has no key %s", mg.GetName(), key)
	}

	return string(content), nil
}
