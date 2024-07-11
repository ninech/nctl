package get

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/gobuffalo/flect"
	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Cmd struct {
	Output              output                `help:"Configures list output. ${enum}" short:"o" enum:"full,no-header,contexts,yaml" default:"full"`
	AllProjects         bool                  `help:"apply the get over all projects." short:"A"`
	Clusters            clustersCmd           `cmd:"" group:"infrastructure.nine.ch" help:"Get Kubernetes Clusters."`
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
)

type listOpt func(cmd *Cmd)

func matchName(name string) listOpt {
	return func(cmd *Cmd) {
		if len(name) == 0 {
			return
		}
		cmd.opts = append(cmd.opts, runtimeclient.MatchingFields{"metadata.name": name})
	}
}

func matchLabel(k, v string) listOpt {
	return func(cmd *Cmd) {
		cmd.opts = append(cmd.opts, runtimeclient.MatchingLabels{k: v})
	}
}

func (cmd *Cmd) list(ctx context.Context, client *api.Client, list runtimeclient.ObjectList, opts ...listOpt) error {
	for _, opt := range opts {
		opt(cmd)
	}

	if !cmd.AllProjects {
		cmd.opts = append(cmd.opts, runtimeclient.InNamespace(client.Project))
	}

	return client.List(ctx, list, cmd.opts...)
}

// writeHeader writes the header row, prepending the project row if
// cmd.AllProjects is set.
func (cmd *Cmd) writeHeader(w io.Writer, headings ...string) {
	if cmd.AllProjects {
		headings = append([]string{"PROJECT"}, headings...)
	}
	cmd.writeTabRow(w, "", headings...)
}

// writeTabRow writes a row to w, prepending the project if
// cmd.AllProjects is set and the project is not empty.
func (cmd *Cmd) writeTabRow(w io.Writer, project string, row ...string) {
	if cmd.AllProjects && len(project) != 0 {
		row = append([]string{project}, row...)
	}

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

func printEmptyMessage(out io.Writer, kind, project string) {
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
	cfg, err := auth.ReadConfig(client.KubeconfigPath, client.KubeconfigContext)
	if err != nil {
		if auth.IsConfigNotFoundError(err) {
			return nil, auth.ReloginNeeded(err)
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
