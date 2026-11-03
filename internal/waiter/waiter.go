// Package waiter watches API objects until they reach a desired state,
// showing the progress with a spinner.
package waiter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/theckman/yacspin"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ResultFunc is the function called on a watch event during a wait.
// It should return true whenever the wait can be considered done.
type ResultFunc func(watch.Event) (bool, error)

// Waiter runs wait stages for a single object.
// Stages that leave Kind or ListOpts empty
// default to the Kind and the Object of the Waiter.
type Waiter struct {
	format.Writer

	Client *api.Client
	Kind   string
	Object runtimeclient.Object
}

// Stage is a single wait for an object list watch to report a desired state.
type Stage struct {
	writer format.Writer

	Kind           string
	WaitMessage    *Message
	DoneMessage    *Message
	ObjectList     runtimeclient.ObjectList
	ListOpts       []runtimeclient.ListOption
	OnResult       ResultFunc
	spinner        *yacspin.Spinner
	DisableSpinner bool
	startTime      time.Time
	// ContinueHint tells the user how to follow an operation which continues
	// on the server after the wait ended, e.g. by a timeout or an interrupt.
	ContinueHint string
	// BeforeWait is a hook that is called just before the wait is being run.
	BeforeWait func()
	// AfterWait is a hook that is called after the wait to clean up.
	AfterWait func()
}

// Message is a progress message shown during or after a stage.
type Message struct {
	Icon     string
	Text     string
	Disabled bool
}

// watchBackoff shapes the reconnect delays after a dropped watch. Steps is the
// number of drops in a row before the wait fails.
var watchBackoff = wait.Backoff{
	Steps:    8,
	Duration: 250 * time.Millisecond,
	Factor:   2.0,
	Jitter:   0.1,
	Cap:      15 * time.Second,
}

const remainingTimeUpdateInterval = time.Second

// Progress returns the message formatted as progress,
// or "" if it is disabled.
func (m *Message) Progress() string {
	if m.Disabled {
		return ""
	}

	return format.Progress(m.Icon, m.Text)
}

// progressWithRemaining returns the progress message
// with the remaining time from the context deadline appended.
func (w *Stage) progressWithRemaining(ctx context.Context) string {
	text := w.WaitMessage.Text
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 {
			text += fmt.Sprintf(" (%s)", remaining.Truncate(time.Second))
		}
	}
	return format.Progress(w.WaitMessage.Icon, text)
}

// Wait runs the stages one after the other
// until all are done, one of them fails or ctx is done.
func (wa *Waiter) Wait(ctx context.Context, stages ...Stage) error {
	for _, stage := range stages {
		if stage.AfterWait != nil {
			defer stage.AfterWait()
		}

		stage.setDefaults(wa)
		stage.writer = wa.Writer

		spinner, err := wa.Spinner(
			stage.progressWithRemaining(ctx),
			stage.progressWithRemaining(ctx),
		)
		if err != nil {
			return err
		}
		stage.spinner = spinner

		if stage.BeforeWait != nil {
			stage.BeforeWait()
		}

		stage.startTime = time.Now()
		if err := retryWatch(ctx, watchBackoff, func() error {
			return stage.wait(ctx, wa.Client)
		}); err != nil {
			_ = stage.spinner.StopFail()
			_ = stage.spinner.Stop()
			return err
		}
	}

	return nil
}

func (w *Stage) setDefaults(wa *Waiter) {
	if len(w.Kind) == 0 {
		w.Kind = wa.Kind
	}

	if w.WaitMessage == nil {
		w.WaitMessage = &Message{
			Text: fmt.Sprintf("waiting for %s to be ready", w.Kind),
			Icon: "⏳",
		}
	}

	if w.DoneMessage == nil {
		w.DoneMessage = &Message{
			Text: fmt.Sprintf("%s ready", w.Kind),
			Icon: "🛫",
		}
	}

	if len(w.ListOpts) == 0 {
		w.ListOpts = []runtimeclient.ListOption{
			runtimeclient.InNamespace(wa.Object.GetNamespace()),
			runtimeclient.MatchingFields{"metadata.name": wa.Object.GetName()},
		}
	}
}

type watchError struct {
	kind string
}

func (werr watchError) Error() string {
	return fmt.Sprintf(
		"error watching %s, the API might be experiencing connectivity issues",
		werr.kind,
	)
}

func isWatchError(err error) bool {
	return errors.As(err, &watchError{})
}

// retryWatch runs watch until it ends with something else than a watchError.
// A watch which stayed open for at least backoff.Cap refreshes the budget.
// wait.Backoff ends the steps as soon as a delay reaches its cap, so the drops
// are counted separately.
func retryWatch(ctx context.Context, backoff wait.Backoff, watch func() error) error {
	delays := backoff
	drops := 0
	for {
		start := time.Now()
		err := watch()
		if !isWatchError(err) {
			return err
		}
		if time.Since(start) >= backoff.Cap {
			delays = backoff
			drops = 0
		}
		if drops++; drops >= backoff.Steps {
			return err
		}
		select {
		case <-time.After(delays.Step()):
		case <-ctx.Done():
			// the next watch reports how the wait ended
		}
	}
}

func (w *Stage) wait(ctx context.Context, client *api.Client) error {
	if !w.DisableSpinner {
		_ = w.spinner.Start()
	}

	return w.watch(ctx, client)
}

func (w *Stage) watch(ctx context.Context, client *api.Client) error {
	if ctx.Err() != nil {
		return w.ended(ctx)
	}

	wa, err := client.Watch(ctx, w.ObjectList, w.ListOpts...)
	if err != nil {
		if ctx.Err() != nil {
			return w.ended(ctx)
		}
		return watchError{kind: w.Kind}
	}
	defer wa.Stop()

	ticker := time.NewTicker(remainingTimeUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case res := <-wa.ResultChan():
			if res.Type == watch.Error || res.Type == "" {
				return watchError{kind: w.Kind}
			}

			done, err := w.OnResult(res)
			if err != nil {
				_ = w.spinner.StopFail()
				return err
			}

			if done {
				elapsed := time.Since(w.startTime).Truncate(time.Second)
				w.spinner.StopMessage(w.WaitMessage.Progress())
				_ = w.spinner.Stop()
				w.writer.Successf(w.DoneMessage.Icon, "%s (%s)", w.DoneMessage.Text, elapsed)

				return nil
			}
		case <-ticker.C:
			w.spinner.Message(w.progressWithRemaining(ctx))
		case <-ctx.Done():
			return w.ended(ctx)
		}
	}
}

// ended reports how the wait ended once ctx is done. A cancelled wait is not
// an error.
func (w *Stage) ended(ctx context.Context) error {
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		_ = w.spinner.StopFail()
		if w.ContinueHint != "" {
			w.writer.Infof("💡", "%s", w.ContinueHint)
		}
		return nil
	}

	msg := "timeout waiting for %s"
	w.spinner.StopFailMessage(format.Progressf("", msg, w.Kind))
	_ = w.spinner.StopFail()

	if w.ContinueHint != "" {
		return fmt.Errorf(msg+"; %s", w.Kind, w.ContinueHint)
	}
	return fmt.Errorf(msg, w.Kind)
}
