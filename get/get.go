package get

import (
	"context"
	"fmt"

	"github.com/gobuffalo/flect"
	"github.com/ninech/nctl/api"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Cmd struct {
	Output             output                `help:"Configures list output. ${enum}" short:"o" enum:"full,no-header,contexts" default:"full"`
	AllNamespaces      bool                  `help:"apply the get over all namespaces." short:"A"`
	Clusters           clustersCmd           `cmd:"" help:"Get Kubernetes Clusters."`
	APIServiceAccounts apiServiceAccountsCmd `cmd:"" name:"apiserviceaccounts" aliases:"asa" help:"Get API Service Accounts"`
}

type output string

const (
	full     output = "full"
	noHeader output = "no-header"
	contexts output = "contexts"
)

func list(ctx context.Context, client *api.Client, list runtimeclient.ObjectList, all bool) error {
	listOpts := []runtimeclient.ListOption{}
	if !all {
		listOpts = append(listOpts, runtimeclient.InNamespace(client.Namespace))
	}

	return client.List(ctx, list, listOpts...)
}

func printEmptyMessage(kind, namespace string) {
	if namespace == "" {
		fmt.Printf("no %s found\n", flect.Pluralize(kind))
		return
	}

	fmt.Printf("no %s found in namespace %s\n", flect.Pluralize(kind), namespace)
}
