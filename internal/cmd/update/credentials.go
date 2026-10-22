package update

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/internal/waiter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// CredentialsResetFlags are the flags to reset the credentials of a resource
// whose controller generates new credentials
// when its connection secret is deleted.
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
	old, err := u.client.RotateCredentials(ctx, u.mg)
	if err != nil {
		return fmt.Errorf("unable to reset the credentials of %s %q: %w", u.kind, u.mg.GetName(), err)
	}

	if !f.Wait {
		u.Successf("🔑", "credentials reset of %s %q started", u.kind, u.mg.GetName())
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, f.WaitTimeout)
	defer cancel()

	// The reference is set, as the rotation succeeded.
	ref := u.mg.GetWriteConnectionSecretToReference()
	list, err := newListFor(u.client.Scheme(), u.mg)
	if err != nil {
		return err
	}

	// created is the creation time of the new secret.
	// The resource has applied the new credentials
	// as soon as it is ready again after that.
	var created metav1.Time
	w := &waiter.Waiter{
		Writer: u.Writer,
		Client: u.client,
		Kind:   u.kind,
		Object: u.mg,
	}
	if err := w.Wait(ctx,
		waiter.Stage{
			Kind:       "credentials",
			ObjectList: &corev1.SecretList{},
			ListOpts: []runtimeclient.ListOption{
				runtimeclient.InNamespace(ref.Namespace),
				runtimeclient.MatchingFields{"metadata.name": ref.Name},
			},
			WaitMessage: &waiter.Message{Icon: "⏳", Text: fmt.Sprintf("waiting for new credentials of %s %q", u.kind, u.mg.GetName())},
			DoneMessage: &waiter.Message{Icon: "🔑", Text: fmt.Sprintf("new credentials of %s %q generated", u.kind, u.mg.GetName())},
			OnResult:    newSecretWritten(old, &created),
		},
		waiter.Stage{
			ObjectList:  list,
			WaitMessage: &waiter.Message{Icon: "⏳", Text: fmt.Sprintf("waiting for %s %q to apply the new credentials", u.kind, u.mg.GetName())},
			DoneMessage: &waiter.Message{Icon: "🔑", Text: fmt.Sprintf("credentials of %s %q reset", u.kind, u.mg.GetName())},
			OnResult:    readySince(&created),
		},
	); err != nil {
		return err
	}

	// the command is unknown if the update did not run through Kong.
	if f.command == "" {
		return nil
	}
	// the project is always passed,
	// as the update might not have been run in the default project.
	u.Printf("You can retrieve the new credentials with:\n\n nctl get %s %s --project %s --print-connection-string\n",
		f.command, u.mg.GetName(), u.mg.GetNamespace(),
	)
	return nil
}

// newSecretWritten returns a [waiter.ResultFunc]
// which is done as soon as a secret other than the one with the old UID exists.
// An empty old UID accepts any secret.
// It stores the creation time of the new secret in created.
func newSecretWritten(old types.UID, created *metav1.Time) waiter.ResultFunc {
	return func(event watch.Event) (bool, error) {
		if event.Type == watch.Deleted {
			return false, nil
		}
		secret, ok := event.Object.(*corev1.Secret)
		if !ok || secret.UID == old {
			return false, nil
		}
		*created = secret.CreationTimestamp
		return true, nil
	}
}

// readySince returns a [waiter.ResultFunc]
// which is done as soon as the resource became available at or after since.
// The controller marks the resource as not ready
// until it has applied the new credentials,
// so the last transition of the ready condition
// tells apart the ready state from before the reset.
// The comparison includes since, as the API stores the times in seconds.
func readySince(since *metav1.Time) waiter.ResultFunc {
	return func(event watch.Event) (bool, error) {
		if event.Type == watch.Deleted {
			return false, nil
		}
		mg, ok := event.Object.(resource.Managed)
		if !ok {
			return false, nil
		}
		ready := mg.GetCondition(runtimev1.TypeReady)
		return ready.Status == corev1.ConditionTrue &&
			ready.Reason == runtimev1.ReasonAvailable &&
			!ready.LastTransitionTime.Before(since), nil
	}
}

// newListFor returns an empty list of the kind of obj.
func newListFor(scheme *runtime.Scheme, obj runtime.Object) (runtimeclient.ObjectList, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return nil, err
	}
	l, err := scheme.New(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	if err != nil {
		return nil, err
	}
	list, ok := l.(runtimeclient.ObjectList)
	if !ok {
		return nil, fmt.Errorf("%T is not a list", l)
	}
	return list, nil
}
