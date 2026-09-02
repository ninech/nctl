package completion

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/posener/complete"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// instanceDatabases completes database names from a database instance status.
type instanceDatabases struct {
	client   clientFunc
	project  projectFinder
	gvk      schema.GroupVersionKind
	argFlags []string
}

func newInstanceDatabases(client clientFunc, project projectFinder, gvk schema.GroupVersionKind) *instanceDatabases {
	return &instanceDatabases{client: client, project: project, gvk: gvk}
}

// withArgFlags implements [argFlagScoped].
func (d *instanceDatabases) withArgFlags(argFlags []string) complete.Predictor {
	scoped := *d
	scoped.argFlags = argFlags

	return &scoped
}

func (d *instanceDatabases) Predict(args complete.Args) []string {
	name := firstPositionalArg(args.Completed, d.argFlags)
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

// firstPositionalArg returns the first positional argument in args.
func firstPositionalArg(args []string, argFlags []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Kong parses everything after "--" as positional.
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
		// A flag assigning its value needs no value token of its own.
		if !strings.Contains(arg, "=") && slices.Contains(argFlags, arg) {
			i++
		}
	}
	return ""
}
