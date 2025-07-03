package get

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/gobuffalo/flect"
	"github.com/ninech/nctl/api"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Cmd struct {
	Output              output                `help:"Configures list output. ${enum}" short:"o" enum:"full,no-header,contexts,yaml,stats,json" default:"full"`
	AllProjects         bool                  `help:"apply the get over all projects." short:"A"`
	AllNamespaces       bool                  `help:"apply the get over all namespaces." hidden:""`
	Clusters            clustersCmd           `cmd:"" group:"infrastructure.nine.ch" aliases:"cluster,vcluster" help:"Get Kubernetes Clusters."`
	APIServiceAccounts  apiServiceAccountsCmd `cmd:"" group:"iam.nine.ch" name:"apiserviceaccounts" aliases:"asa" help:"Get API Service Accounts."`
	Projects            projectCmd            `cmd:"" group:"management.nine.ch" name:"projects" aliases:"proj" help:"Get Projects."`
	Applications        applicationsCmd       `cmd:"" group:"deplo.io" name:"applications" aliases:"app,apps,application" help:"Get deplo.io Applications."`
	Builds              buildCmd              `cmd:"" group:"deplo.io" name:"builds" aliases:"build" help:"Get deplo.io Builds."`
	Releases            releasesCmd           `cmd:"" group:"deplo.io" name:"releases" aliases:"release" help:"Get deplo.io Releases."`
	Configs             configsCmd            `cmd:"" group:"deplo.io" name:"configs" aliases:"config" help:"Get deplo.io Project Configuration."`
	MySQL               mySQLCmd              `cmd:"" group:"storage.nine.ch" name:"mysql" help:"Get MySQL instances."`
	MySQLDatabases      mysqlDatabaseCmd      `cmd:"" group:"storage.nine.ch" name:"mysqldatabases" aliases:"mysqldatabase" help:"Get MySQL databases."`
	Postgres            postgresCmd           `cmd:"" group:"storage.nine.ch" name:"postgres" help:"Get PostgreSQL instances."`
	PostgresDatabases   postgresDatabaseCmd   `cmd:"" group:"storage.nine.ch" name:"postgresdatabases" aliases:"postgresdatabase" help:"Get PostgreSQL databases."`
	KeyValueStore       keyValueStoreCmd      `cmd:"" group:"storage.nine.ch" name:"keyvaluestore" aliases:"kvs" help:"Get KeyValueStore instances."`
	All                 allCmd                `cmd:"" name:"all" help:"Get project content"`
	CloudVirtualMachine cloudVMCmd            `cmd:"" group:"infrastructure.nine.ch" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Get a CloudVM."`
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
	jsonOut  output = "json"
)

func (cmd *Cmd) list(ctx context.Context, client *api.Client, list runtimeclient.ObjectList, opts ...api.ListOpt) error {
	if cmd.AllProjects {
		opts = append(opts, api.AllProjects())
	}
	if cmd.AllNamespaces {
		opts = append(opts, api.AllNamespaces())
	}
	return client.ListObjects(ctx, list, opts...)
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
	if cmd.Output == jsonOut {
		fmt.Fprintf(defaultOut(out), "[]")
		return
	}
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

func getConnectionSecretMap(ctx context.Context, client *api.Client, mg resource.Managed) (map[string][]byte, error) {
	secret, err := client.GetConnectionSecret(ctx, mg)
	if err != nil {
		return nil, err
	}

	return secret.Data, nil
}

func getConnectionSecret(ctx context.Context, client *api.Client, key string, mg resource.Managed) (string, error) {
	secrets, err := getConnectionSecretMap(ctx, client, mg)
	if err != nil {
		return "", fmt.Errorf("unable to get connection secret: %w", err)
	}

	content, ok := secrets[key]
	if !ok {
		return "", fmt.Errorf("secret %s has no key %s", mg.GetName(), key)
	}

	return string(content), nil
}

func (cmd *resourceCmd) printSecret(out io.Writer, ctx context.Context, client *api.Client, mg resource.Managed, field func(string, string) string) error {
	secrets, err := getConnectionSecretMap(ctx, client, mg)
	if err != nil {
		return err
	}

	for k, v := range secrets {
		fmt.Fprintln(out, field(k, string(v)))
		break
	}

	return nil
}
