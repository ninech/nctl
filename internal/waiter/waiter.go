// Package waiter watches API objects until they reach a desired state,
// showing the progress with a spinner.
package waiter

import (
	"context"
	"fmt"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/theckman/yacspin"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
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

var watchBackoff = wait.Backoff{
	Steps:    15,
	Duration: 10 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
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
		if err := retry.OnError(watchBackoff, isWatchError, func() error {
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
	_, ok := err.(watchError)
	return ok
}

func (w *Stage) wait(ctx context.Context, client *api.Client) error {
	if !w.DisableSpinner {
		_ = w.spinner.Start()
	}

	return w.watch(ctx, client)
}

func (w *Stage) watch(ctx context.Context, client *api.Client) error {
	wa, err := client.Watch(ctx, w.ObjectList, w.ListOpts...)
	if err != nil {
		if err == context.Canceled {
			return err
		}
		return watchError{kind: w.Kind}
	}

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
				wa.Stop()
				elapsed := time.Since(w.startTime).Truncate(time.Second)
				w.spinner.StopMessage(w.WaitMessage.Progress())
				_ = w.spinner.Stop()
				w.writer.Successf(w.DoneMessage.Icon, "%s (%s)", w.DoneMessage.Text, elapsed)

				return nil
			}
		case <-ticker.C:
			w.spinner.Message(w.progressWithRemaining(ctx))
		case <-ctx.Done():
			switch ctx.Err() {
			case context.DeadlineExceeded:
				msg := "timeout waiting for %s"
				w.spinner.StopFailMessage(format.Progressf("", msg, w.Kind))
				_ = w.spinner.StopFail()

				return fmt.Errorf(msg, w.Kind)
			case context.Canceled:
				_ = w.spinner.StopFail()
				return nil
			}
		}
	}
}
