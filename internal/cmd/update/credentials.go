package update

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/internal/waiter"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CredentialsResetFlags are the flags to reset the credentials of a resource
// whose controller generates new credentials
// when its connection secret is deleted.
//
// It has to be exported so that Kong picks up the flags of the embedded struct.
type CredentialsResetFlags struct {
	ResetCredentials bool          `help:"Generate new credentials. The current credentials stop working, every client using them has to be updated."`
	Force            bool          `help:"Do not ask for confirmation before resetting the credentials."`
	Wait             bool          `default:"true" help:"Wait until the new credentials are available. Only relevant if reset-credentials is set."`
	WaitTimeout      time.Duration `default:"5m" help:"Duration to wait for the new credentials. Only relevant if reset-credentials and wait are set."`

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

// reset deletes the connection secret of the resource,
// which makes the controller generate new credentials
// and write them to a new secret.
func (f *CredentialsResetFlags) reset(ctx context.Context, u *updater) error {
	ref := u.mg.GetWriteConnectionSecretToReference()
	if ref == nil {
		return fmt.Errorf("%s %q has no connection secret, its credentials can not be reset", u.kind, u.mg.GetName())
	}

	secret := &corev1.Secret{}
	if err := u.client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, secret); err != nil {
		// a missing secret is being regenerated already,
		// so we just wait for the new one.
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("unable to get the connection secret of %s %q: %w", u.kind, u.mg.GetName(), err)
		}
	} else if err := u.client.Delete(ctx, secret, runtimeclient.Preconditions{UID: &secret.UID}); runtimeclient.IgnoreNotFound(err) != nil {
		return fmt.Errorf("unable to delete the connection secret of %s %q: %w", u.kind, u.mg.GetName(), err)
	}

	if !f.Wait {
		u.Successf("🔑", "credentials reset of %s %q started", u.kind, u.mg.GetName())
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, f.WaitTimeout)
	defer cancel()

	w := &waiter.Waiter{
		Writer: u.Writer,
		Client: u.client,
		Kind:   "credentials",
		Object: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace}},
	}
	if err := w.Wait(ctx, waiter.Stage{
		ObjectList:  &corev1.SecretList{},
		WaitMessage: &waiter.Message{Icon: "⏳", Text: fmt.Sprintf("waiting for new credentials of %s %q", u.kind, u.mg.GetName())},
		DoneMessage: &waiter.Message{Icon: "🔑", Text: fmt.Sprintf("credentials of %s %q reset", u.kind, u.mg.GetName())},
		OnResult:    newSecretWritten(secret.UID),
	}); err != nil {
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
func newSecretWritten(old types.UID) waiter.ResultFunc {
	return func(event watch.Event) (bool, error) {
		if event.Type == watch.Deleted {
			return false, nil
		}
		secret, ok := event.Object.(*corev1.Secret)
		if !ok {
			return false, nil
		}
		return secret.UID != old, nil
	}
}
