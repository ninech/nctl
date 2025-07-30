// Package get implements the get command.
package get

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/gobuffalo/flect"
	"github.com/liggitt/tabwriter"
	"github.com/ninech/nctl/api"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Cmd struct {
	output
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
	All                 allCmd                `cmd:"" name:"all" help:"Get project content."`
	CloudVirtualMachine cloudVMCmd            `cmd:"" group:"infrastructure.nine.ch" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Get a CloudVM."`
	ServiceConnection   serviceConnectionCmd  `cmd:"" group:"networking.nine.ch" name:"serviceconnection" aliases:"sc" help:"Get a ServiceConnection."`
}

type output struct {
	Format        outputFormat `help:"Configures list output. ${enum}" name:"output" short:"o" enum:"full,no-header,contexts,yaml,stats,json" default:"full"`
	AllProjects   bool         `help:"apply the get over all projects." short:"A" xor:"watch"`
	AllNamespaces bool         `help:"apply the get over all namespaces." hidden:"" xor:"watch"`
	Watch         bool         `help:"Watch resource(s) for changes and print the updated resource." short:"w" xor:"watch"`
	tabWriter     *tabwriter.Writer
	writer        io.Writer
}

type resourceCmd struct {
	Name string `arg:"" predictor:"resource_name" help:"Name of the resource to get. If omitted all in the project will be listed." default:""`
}

type outputFormat string

const (
	full     outputFormat = "full"
	noHeader outputFormat = "no-header"
	contexts outputFormat = "contexts"
	yamlOut  outputFormat = "yaml"
	stats    outputFormat = "stats"
	jsonOut  outputFormat = "json"
)

func (cmd *Cmd) AfterApply() error {
	cmd.initOut()
	return nil
}

// listPrinter needs to be implemented by all resources.
type listPrinter interface {
	print(context.Context, *api.Client, runtimeclient.ObjectList, *output) error
	list() runtimeclient.ObjectList
}

func (cmd *Cmd) listPrint(ctx context.Context, client *api.Client, lp listPrinter, opts ...api.ListOpt) error {
	if cmd.AllProjects {
		opts = append(opts, api.AllProjects())
	}
	if cmd.AllNamespaces {
		opts = append(opts, api.AllNamespaces())
	}
	if cmd.Watch {
		opts = append(opts, api.Watch(func(list runtimeclient.ObjectList) error {
			return lp.print(ctx, client, list, &cmd.output)
		}))
	}
	list := lp.list()
	if err := client.ListObjects(ctx, list, opts...); err != nil {
		return err
	}
	if cmd.Watch {
		return nil
	}
	return lp.print(ctx, client, list, &cmd.output)
}

// writeHeader writes the header row, prepending the always shown project
func (out *output) writeHeader(headings ...string) {
	out.initOut()
	// don't write header if watch is enabled and RememberedWidths is not empty,
	// as it means we have already printed the header.
	if out.Watch && len(out.tabWriter.RememberedWidths()) != 0 {
		return
	}
	out.writeTabRow("PROJECT", headings...)
}

// writeTabRow writes a row to w, prepending the passed project
func (out *output) writeTabRow(project string, row ...string) {
	out.initOut()

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
		fmt.Fprintf(out.tabWriter, "%s\n", row[0])
	default:
		fmt.Fprintf(out.tabWriter, "%s", row[0])
		for _, r := range row[1:] {
			fmt.Fprintf(out.tabWriter, "\t%s", r)
		}
		fmt.Fprint(out.tabWriter, "\n")
	}
}

func (out *output) printEmptyMessage(kind, project string) error {
	out.initOut()

	if out.Format == jsonOut {
		_, err := fmt.Fprintf(out.writer, "[]")
		return err
	}
	if out.AllProjects {
		_, err := fmt.Fprintf(out.writer, "no %s found in any project\n", flect.Pluralize(kind))
		return err
	}
	if project == "" {
		_, err := fmt.Fprintf(out.writer, "no %s found\n", flect.Pluralize(kind))
		return err
	}

	_, err := fmt.Fprintf(out.writer, "no %s found in project %s\n", flect.Pluralize(kind), project)
	return err
}

func (out *output) initOut() {
	if out.writer == nil {
		out.writer = os.Stdout
	}

	if out.tabWriter == nil {
		out.tabWriter = tabwriter.NewWriter(out.writer, 0, 0, 4, ' ', tabwriter.RememberWidths)
	}
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
		_, err = fmt.Fprintln(out, field(k, string(v)))
		return err
	}

	return nil
}

func printBase64(out io.Writer, s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	pem, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("unable to decode base64: %w", err)
	}

	_, err = fmt.Fprintln(out, string(bytes.TrimSpace(pem)))
	return err
}
