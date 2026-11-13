package update

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/waiter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// CredentialsResetFlags are the flags to reset the credentials of a resource
// whose controller supports [api.RotateCredentialsAnnotation].
// Set credentials_wait_timeout to change the default of the wait timeout,
// e.g. for resources which take longer to apply the new credentials.
//
// It has to be exported so that Kong picks up the flags of the embedded struct.
type CredentialsResetFlags struct {
	ResetCredentials bool          `help:"Generate new credentials. The current credentials stop working, every client using them has to be updated."`
	Force            bool          `help:"Do not ask for confirmation before resetting the credentials."`
	Wait             bool          `default:"true" help:"Wait until the new credentials are generated and applied to the resource. Only relevant if reset-credentials is set."`
	WaitTimeout      time.Duration `default:"${credentials_wait_timeout=5m}" help:"Duration to wait for the new credentials. Only relevant if reset-credentials and wait are set."`

	// command is the name of the selected update sub-command,
	// which is also the name of the get sub-command of the resource.
	command string
}

// BeforeApply stores the name of the selected update sub-command
// to suggest the get sub-command retrieving the new credentials.
func (f *CredentialsResetFlags) BeforeApply(kctx *kong.Context) error {
	if node := kctx.Selected(); node != nil {
		f.command = node.Name
	}
	return nil
}

// withCredentialsReset makes u reset the credentials after the update
// if requested by f.
func (u *updater) withCredentialsReset(f *CredentialsResetFlags) *updater {
	if f.ResetCredentials {
		u.credentialsReset = f
	}
	return u
}

func (f *CredentialsResetFlags) confirm(u *updater) (bool, error) {
	if f.Force {
		return true, nil
	}

	return u.Confirm(u.Reader, fmt.Sprintf(
		"Do you really want to reset the credentials of %s %q? Clients using the current credentials will lose access.",
		u.kind, u.mg.GetName(),
	))
}

// reset makes the controller of the resource generate new credentials,
// see [api.Client.RotateCredentials],
// and waits until the resource has applied them if requested by f.
func (f *CredentialsResetFlags) reset(ctx context.Context, u *updater) error {
	token, err := u.client.RotateCredentials(ctx, u.mg)
	if err != nil {
		return fmt.Errorf("unable to reset the credentials of %s %q: %w", u.kind, u.mg.GetName(), err)
	}

	if !f.Wait {
		u.Successf("🔑", "credentials reset of %s %q started", u.kind, u.mg.GetName())
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, f.WaitTimeout)
	defer cancel()

	// The typed status of the resource might not know
	// the credentials rotation fields yet, which would drop them.
	list, err := newUnstructuredListFor(u.client.Scheme(), u.mg)
	if err != nil {
		return err
	}

	progress := &rotationProgress{token: token}
	w := &waiter.Waiter{
		Writer: u.Writer,
		Client: u.client,
		Kind:   u.kind,
		Object: u.mg,
	}
	if err := w.Wait(ctx, waiter.Stage{
		ObjectList:  list,
		WaitMessage: &waiter.Message{Icon: "⏳", Text: fmt.Sprintf("waiting for new credentials of %s %q", u.kind, u.mg.GetName())},
		DoneMessage: &waiter.Message{Icon: "🔑", Text: fmt.Sprintf("new credentials of %s %q generated and applied", u.kind, u.mg.GetName())},
		OnResult:    progress.onResult,
	}); err != nil {
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return err
		}
		if !progress.acknowledged {
			return fmt.Errorf("the API did not acknowledge the credentials reset of %s %q, "+
				"it might not support resetting the credentials yet: %w",
				u.kind, u.mg.GetName(), err,
			)
		}
		return fmt.Errorf("the credentials of %s %q have been reset, "+
			"but the new credentials have not been applied yet, the reset continues in the background: %w",
			u.kind, u.mg.GetName(), err,
		)
	}

	// the command is unknown if the update did not run through Kong.
	if f.command == "" {
		return nil
	}
	// the project is always passed,
	// as the update might not have been run in the default project.
	u.Printf("You can retrieve the new credentials with:\n\n nctl get %s %s --project %s %s\n",
		f.command, u.mg.GetName(), u.mg.GetNamespace(), printCredentialsFlag(f.command),
	)
	return nil
}

// printCredentialsFlag returns the flag of the get sub-command
// which prints the credentials of the resource.
func printCredentialsFlag(command string) string {
	if command == "bucketuser" {
		return "--print-credentials"
	}
	return "--print-connection-string"
}

// rotationProgress tracks the credentials reset requested with token
// through the watch events of the resource.
type rotationProgress struct {
	token string
	// acknowledged is set as soon as the controller has reset the credentials
	// for token, i.e. stored it in status.atProvider.lastCredentialsRotation.
	acknowledged bool
}

// onResult is a [waiter.ResultFunc] which is done
// as soon as the controller has generated and applied new credentials for the token.
// The controller sets lastCredentialsRotation and resettingCredentials
// in the same status update,
// so the token together with resettingCredentials being false
// only appears once the reset is done.
// It fails if the annotation has been set to another value in the meantime,
// as the reset of the token is superseded then.
func (p *rotationProgress) onResult(event watch.Event) (bool, error) {
	if event.Type == watch.Deleted {
		return false, nil
	}
	obj, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(event.Object)
		if err != nil {
			return false, nil
		}
		obj = &unstructured.Unstructured{Object: content}
	}

	if requested := obj.GetAnnotations()[api.RotateCredentialsAnnotation]; requested != p.token {
		return false, fmt.Errorf("the credentials reset was superseded by another one requested at %q, "+
			"the new credentials are generated in the background", requested)
	}

	last, _, err := unstructured.NestedString(obj.Object, "status", "atProvider", "lastCredentialsRotation")
	if err != nil || last != p.token {
		return false, nil
	}
	p.acknowledged = true

	resetting, _, err := unstructured.NestedBool(obj.Object, "status", "atProvider", "resettingCredentials")
	if err != nil {
		return false, nil
	}
	return !resetting, nil
}

// newUnstructuredListFor returns an empty unstructured list of the kind of obj.
func newUnstructuredListFor(scheme *runtime.Scheme, obj runtime.Object) (*unstructured.UnstructuredList, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return nil, err
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	return list, nil
}
