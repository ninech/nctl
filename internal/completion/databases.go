package completion

import (
	"context"
	"fmt"
	"strings"

	"github.com/posener/complete"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// instanceDatabases completes database names from a database instance status.
type instanceDatabases struct {
	client  clientFunc
	project projectFinder
	gvk     schema.GroupVersionKind
}

func newInstanceDatabases(client clientFunc, project projectFinder, gvk schema.GroupVersionKind) *instanceDatabases {
	return &instanceDatabases{client: client, project: project, gvk: gvk}
}

func (d *instanceDatabases) Predict(args complete.Args) []string {
	name := firstPositionalArg(args.Completed)
	if name == "" {
		return nil
	}

	p, incomplete := d.project.find(args)
	if incomplete {
		return nil
	}

	client, err := d.client()
	if err != nil {
		return fail(err)
	}

	ns := client.Project
	if p != "" {
		ns = p
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(d.gvk)

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	if err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, u); err != nil {
		return fail(fmt.Errorf("cannot get %s %q in project %q: %w", d.gvk.Kind, name, ns, err))
	}

	databases, found, err := unstructured.NestedMap(u.Object, "status", "atProvider", "databases")
	if err != nil {
		return fail(fmt.Errorf("%s %q reports no readable databases: %w", d.gvk.Kind, name, err))
	}
	// An instance without any database yet is not an error.
	if !found {
		return nil
	}

	names := make([]string, 0, len(databases))
	for dbName := range databases {
		names = append(names, dbName)
	}
	return names
}

// firstPositionalArg returns the first positional argument in args, skipping flag-value pairs.
func firstPositionalArg(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if !strings.Contains(arg, "=") {
				i++ // skip the following value token
			}
			continue
		}
		return arg
	}
	return ""
}
