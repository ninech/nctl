package get

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/gobuffalo/flect"
	"github.com/ninech/nctl/api"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Cmd struct {
	Output             output                `help:"Configures list output. ${enum}" short:"o" enum:"full,no-header,contexts,yaml" default:"full"`
	AllProjects        bool                  `help:"apply the get over all projects." short:"A"`
	Clusters           clustersCmd           `cmd:"" group:"infrastructure.nine.ch" help:"Get Kubernetes Clusters."`
	APIServiceAccounts apiServiceAccountsCmd `cmd:"" group:"iam.nine.ch" name:"apiserviceaccounts" aliases:"asa" help:"Get API Service Accounts."`
	Projects           projectCmd            `cmd:"" group:"management.nine.ch" name:"projects" aliases:"proj" help:"Get Projects."`
	Applications       applicationsCmd       `cmd:"" group:"deplo.io" name:"applications" aliases:"app,apps" help:"Get deplo.io Applications. (Beta - requires access)"`
	Builds             buildCmd              `cmd:"" group:"deplo.io" name:"builds" aliases:"build" help:"Get deplo.io Builds. (Beta - requires access)"`
	Releases           releasesCmd           `cmd:"" group:"deplo.io" name:"releases" aliases:"release" help:"Get deplo.io Releases. (Beta - requires access)"`
	Configs            configsCmd            `cmd:"" group:"deplo.io" name:"configs" aliases:"config" help:"Get deplo.io Project Configuration. (Beta - requires access)"`

	opts []runtimeclient.ListOption
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
		fmt.Fprintf(w, "%s\t", project)
	}

	format := "%s\t"
	// if there is just one element to be printed, we do not need a tab
	// separator
	if len(row) == 1 {
		format = "%s"
	}
	for _, r := range row {
		fmt.Fprintf(w, format, r)
	}
	fmt.Fprintf(w, "\n")
}

func printEmptyMessage(out io.Writer, kind, project string) {
	if project == "" {
		fmt.Fprintf(defaultOut(out), "no %s found\n", flect.Pluralize(kind))
		return
	}

	fmt.Printf("no %s found in project %s\n", flect.Pluralize(kind), project)
}

func defaultOut(out io.Writer) io.Writer {
	if out == nil {
		return os.Stdout
	}
	return out
}
