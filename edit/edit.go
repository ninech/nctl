// Package edit provides functionality to edit resources in a text editor.
package edit

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Cmd struct {
	VCluster            resourceCmd `cmd:"" group:"infrastructure.nine.ch" name:"vcluster" help:"Edit a vcluster."`
	APIServiceAccount   resourceCmd `cmd:"" group:"iam.nine.ch" name:"apiserviceaccount" aliases:"asa" help:"Edit an API Service Account."`
	Project             resourceCmd `cmd:"" group:"management.nine.ch" name:"project" help:"Edit a project."`
	Config              resourceCmd `cmd:"" group:"deplo.io" name:"config" alias:"projectconfig" help:"Edit a deplo.io Project Configuration."`
	Application         resourceCmd `cmd:"" group:"deplo.io" name:"application" aliases:"app,application" help:"Edit a deplo.io Application."`
	MySQL               resourceCmd `cmd:"" group:"storage.nine.ch" name:"mysql" help:"Edit a MySQL instance."`
	MySQLDatabase       resourceCmd `cmd:"" group:"storage.nine.ch" name:"mysqldatabase" help:"Edit a MySQL database."`
	Postgres            resourceCmd `cmd:"" group:"storage.nine.ch" name:"postgres" help:"Edit a PostgreSQL instance."`
	PostgresDatabase    resourceCmd `cmd:"" group:"storage.nine.ch" name:"postgresdatabase" help:"Edit a PostgreSQL database."`
	KeyValueStore       resourceCmd `cmd:"" group:"storage.nine.ch" name:"keyvaluestore" aliases:"kvs" help:"Edit a KeyValueStore instance."`
	OpenSearch          resourceCmd `cmd:"" group:"storage.nine.ch" name:"opensearch" aliases:"os" help:"Edit an OpenSearch cluster."`
	CloudVirtualMachine resourceCmd `cmd:"" group:"infrastructure.nine.ch" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Edit a CloudVM."`
}

type resourceCmd struct {
	format.Writer

	Name string `arg:"" completion-predictor:"resource_name" help:"Name of the resource to edit." required:""`
}

const header = `# Please edit the %s below.
# Lines beginning with a '#' will be ignored. If an error occurs while
# saving this file will be reopened with the relevant failures. Note
# that the status can't be updated.
#
`

var editorEnvs = []string{"NCTL_EDITOR", "EDITOR"}

func (cmd *resourceCmd) Run(kong *kong.Context, ctx context.Context, c *api.Client) error {
	gvk, err := findGVK(c.Scheme(), append(kong.Selected().Aliases, kong.Selected().Name)...)
	if err != nil {
		return err
	}
	obj := &unstructured.Unstructured{}
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	if err := c.Get(ctx, c.Name(cmd.Name), obj); err != nil {
		return err
	}
	// remove managedfields so they don't pollute the object
	managedFields := obj.GetManagedFields()
	obj.SetManagedFields(nil)

	f, err := os.CreateTemp("", "nctl-edit-*.yaml")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(f.Name())
	}()

	writeHeader(f, obj)
	if err := format.PrettyPrintObjects(
		[]client.Object{obj},
		format.PrintOpts{Out: f, AllFields: true},
	); err != nil {
		return err
	}
	oldModTime, err := modTime(f)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	const tries = 2
	var editError error
	var modified bool
	for range tries {
		if editError != nil {
			if err := writeError(f.Name(), editError, obj); err != nil {
				return err
			}
			editError = nil
		}
		if err := editor.NewDefaultEditor(editorEnvs).Launch(f.Name()); err != nil {
			return err
		}

		f, err = os.Open(f.Name())
		if err != nil {
			return err
		}
		newModTime, err := modTime(f)
		if err != nil {
			return err
		}
		// no need to update the object if the file is unchanged
		if newModTime.Equal(oldModTime) {
			break
		}

		// create new empty object
		obj = &unstructured.Unstructured{}
		if err := yaml.NewYAMLOrJSONDecoder(f, 4096).Decode(obj); err != nil {
			editError = err
			continue
		}
		// restore managedfields so we don't delete them
		obj.SetManagedFields(managedFields)

		oldResourceVersion := obj.GetResourceVersion()
		if err := c.Update(ctx, obj); err != nil {
			editError = err
			continue
		}
		modified = oldResourceVersion != obj.GetResourceVersion()
		break
	}
	if editError != nil {
		return editError
	}
	if modified {
		cmd.Successf("🏗", "updated %s", formatObj(obj))
	} else {
		cmd.Printf("no changes made to %s\n", formatObj(obj))
	}
	return nil
}

func modTime(f *os.File) (time.Time, error) {
	stat, err := f.Stat()
	if err != nil {
		return time.Time{}, err
	}
	return stat.ModTime(), nil
}

func writeHeader(w io.Writer, obj client.Object) {
	fmt.Fprintf(w, header, formatObj(obj))
}

// writeError rewrites the file with an error message after the header.
func writeError(fileName string, editError error, obj client.Object) error {
	f, err := os.OpenFile(fileName, os.O_RDWR, os.ModeAppend)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	var newFileContents bytes.Buffer
	writeHeader(&newFileContents, obj)
	if _, err := newFileContents.WriteString(
		fmt.Sprintf("# %s\n", printStatusErrorDetails(editError)),
	); err != nil {
		return err
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if _, err := newFileContents.WriteString(scanner.Text() + "\n"); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if _, err := newFileContents.WriteTo(f); err != nil {
		return err
	}
	return nil
}

func formatObj(obj client.Object) string {
	return fmt.Sprintf(
		"%s %s/%s",
		obj.GetObjectKind().GroupVersionKind().Kind,
		obj.GetName(),
		obj.GetNamespace(),
	)
}

func findGVK(scheme *runtime.Scheme, names ...string) (schema.GroupVersionKind, error) {
	for gvk := range scheme.AllKnownTypes() {
		if slices.Contains(names, strings.ToLower(gvk.Kind)) {
			return gvk, nil
		}
	}
	return schema.GroupVersionKind{}, fmt.Errorf("no type found for %s", names)
}

// printStatusErrorDetails pretty-prints a [kerrors.StatusError] with all the
// cause details if present.
func printStatusErrorDetails(err error) string {
	s, ok := err.(*kerrors.StatusError)
	if !ok {
		return err.Error()
	}
	if s.Status().Details == nil || len(s.Status().Details.Causes) == 0 {
		return err.Error()
	}
	causes := []string{}
	for _, cause := range s.Status().Details.Causes {
		msg := cause.Message
		if cause.Field != "" && cause.Field != "<nil>" {
			msg = fmt.Sprintf("%s: %s", cause.Field, cause.Message)
		}
		causes = append(causes, fmt.Sprintf("# * %s", msg))
	}
	return fmt.Sprintf(
		"%s %q is invalid:\n%s",
		s.Status().Details.Kind,
		s.Status().Details.Name,
		strings.Join(causes, "\n"),
	)
}
