// Package create provides functionality to create resources in the nine.ch API.
package create

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/lucasepe/codename"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/theckman/yacspin"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Cmd struct {
	Filename            *os.File             `short:"f" help:"Create any resource from a yaml or json file." completion-predictor:"file"`
	FromFile            fromFile             `cmd:"" default:"1" name:"-f <file>" help:"Create any resource from a yaml or json file."`
	VCluster            vclusterCmd          `cmd:"" group:"infrastructure.nine.ch" name:"vcluster" help:"Create a new vcluster."`
	APIServiceAccount   apiServiceAccountCmd `cmd:"" group:"iam.nine.ch" name:"apiserviceaccount" aliases:"asa" help:"Create a new API Service Account."`
	Project             projectCmd           `cmd:"" group:"management.nine.ch" name:"project" help:"Create a new project."`
	Config              configCmd            `cmd:"" group:"deplo.io" name:"config"  help:"Create a new deplo.io Project Configuration."`
	Application         applicationCmd       `cmd:"" group:"deplo.io" name:"application" aliases:"app,application" help:"Create a new deplo.io Application."`
	MySQL               mySQLCmd             `cmd:"" group:"storage.nine.ch" name:"mysql" help:"Create a new MySQL instance."`
	MySQLDatabase       mysqlDatabaseCmd     `cmd:"" group:"storage.nine.ch" name:"mysqldatabase" help:"Create a new MySQL database."`
	Postgres            postgresCmd          `cmd:"" group:"storage.nine.ch" name:"postgres" help:"Create a new PostgreSQL instance."`
	PostgresDatabase    postgresDatabaseCmd  `cmd:"" group:"storage.nine.ch" name:"postgresdatabase" help:"Create a new PostgreSQL database."`
	KeyValueStore       keyValueStoreCmd     `cmd:"" group:"storage.nine.ch" name:"keyvaluestore" aliases:"kvs" help:"Create a new KeyValueStore instance."`
	OpenSearch          openSearchCmd        `cmd:"" group:"storage.nine.ch" name:"opensearch" aliases:"os" help:"Create a new OpenSearch cluster."`
	CloudVirtualMachine cloudVMCmd           `cmd:"" group:"infrastructure.nine.ch" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Create a new CloudVM."`
	ServiceConnection   serviceConnectionCmd `cmd:"" group:"networking.nine.ch" name:"serviceconnection" aliases:"sc" help:"Create a new ServiceConnection."`
	Bucket              bucketCmd            `cmd:"" group:"storage.nine.ch" name:"bucket" help:"Create a new Bucket."`
	BucketUser          bucketUserCmd        `cmd:"" group:"storage.nine.ch" name:"bucketuser" aliases:"bu" help:"Create a new BucketUser."`
}

type resourceCmd struct {
	format.Writer `kong:"-"`
	Name          string        `arg:"" help:"Name of the new resource. A random name is generated if omitted." default:""`
	Wait          bool          `default:"true" help:"Wait until resource is fully created."`
	WaitTimeout   time.Duration `default:"30m" help:"Duration to wait for resource getting ready. Only relevant if wait is set."`
}

// BeforeApply initializes Writer from Kong's bound [io.Writer].
func (cmd *resourceCmd) BeforeApply(writer io.Writer) error {
	return cmd.Writer.BeforeApply(writer)
}

// resultFunc is the function called on a watch event during creation. It
// should return true whenever the wait can be considered done.
type resultFunc func(watch.Event) (bool, error)

type creator struct {
	format.Writer

	client *api.Client
	mg     resource.Managed
	kind   string

	timeout time.Duration
}

type waitStage struct {
	format.Writer

	kind           string
	waitMessage    *message
	doneMessage    *message
	objectList     runtimeclient.ObjectList
	listOpts       []runtimeclient.ListOption
	onResult       resultFunc
	spinner        *yacspin.Spinner
	disableSpinner bool
	startTime      time.Time
	// beforeWait is a hook that is called just before the wait is being run.
	beforeWait func()
	// afterWait is a hook that is called after the wait to clean up.
	afterWait func()
}

type message struct {
	icon     string
	text     string
	disabled bool
}

var watchBackoff = wait.Backoff{
	Steps:    15,
	Duration: 10 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}

const remainingTimeUpdateInterval = time.Second

func (m *message) progress() string {
	if m.disabled {
		return ""
	}

	return format.Progress(m.icon, m.text)
}

// progressWithRemaining returns the progress message with the remaining time
// from the context deadline appended.
func (w *waitStage) progressWithRemaining(ctx context.Context) string {
	text := w.waitMessage.text
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 {
			text += fmt.Sprintf(" (%s)", remaining.Truncate(time.Second))
		}
	}
	return format.Progress(w.waitMessage.icon, text)
}

func (cmd *resourceCmd) newCreator(client *api.Client, mg resource.Managed, kind string) *creator {
	return &creator{client: client, mg: mg, kind: kind, timeout: cmd.WaitTimeout, Writer: cmd.Writer}
}

func (c *creator) createResource(ctx context.Context) error {
	if err := c.client.Create(ctx, c.mg); err != nil {
		return fmt.Errorf("unable to create %s %q: %w", c.kind, c.mg.GetName(), err)
	}

	c.Successf("🏗", "created %s %q in project %q", c.kind, c.mg.GetName(), c.mg.GetNamespace())
	return nil
}

func (c *creator) wait(ctx context.Context, stages ...waitStage) error {
	for _, stage := range stages {
		if stage.afterWait != nil {
			defer stage.afterWait()
		}

		stage.setDefaults(c)
		stage.Writer = c.Writer

		spinner, err := c.Spinner(
			stage.progressWithRemaining(ctx),
			stage.progressWithRemaining(ctx),
		)
		if err != nil {
			return err
		}
		stage.spinner = spinner

		if stage.beforeWait != nil {
			stage.beforeWait()
		}

		stage.startTime = time.Now()
		if err := retry.OnError(watchBackoff, isWatchError, func() error {
			return stage.wait(ctx, c.client)
		}); err != nil {
			_ = stage.spinner.StopFail()
			_ = stage.spinner.Stop()
			return err
		}
	}

	return nil
}

func (w *waitStage) setDefaults(c *creator) {
	if len(w.kind) == 0 {
		w.kind = c.kind
	}

	if w.waitMessage == nil {
		w.waitMessage = &message{
			text: fmt.Sprintf("waiting for %s to be ready", w.kind),
			icon: "⏳",
		}
	}

	if w.doneMessage == nil {
		w.doneMessage = &message{
			text: fmt.Sprintf("%s ready", w.kind),
			icon: "🛫",
		}
	}

	if len(w.listOpts) == 0 {
		w.listOpts = []runtimeclient.ListOption{
			runtimeclient.InNamespace(c.mg.GetNamespace()),
			runtimeclient.MatchingFields{"metadata.name": c.mg.GetName()},
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

func (w *waitStage) wait(ctx context.Context, client *api.Client) error {
	if !w.disableSpinner {
		_ = w.spinner.Start()
	}

	return w.watch(ctx, client)
}

func (w *waitStage) watch(ctx context.Context, client *api.Client) error {
	wa, err := client.Watch(ctx, w.objectList, w.listOpts...)
	if err != nil {
		if err == context.Canceled {
			return err
		}
		return watchError{kind: w.kind}
	}

	ticker := time.NewTicker(remainingTimeUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case res := <-wa.ResultChan():
			if res.Type == watch.Error || res.Type == "" {
				return watchError{kind: w.kind}
			}

			done, err := w.onResult(res)
			if err != nil {
				_ = w.spinner.StopFail()
				return err
			}

			if done {
				wa.Stop()
				elapsed := time.Since(w.startTime).Truncate(time.Second)
				w.spinner.StopMessage(w.waitMessage.progress())
				_ = w.spinner.Stop()
				w.Successf(w.doneMessage.icon, "%s (%s)", w.doneMessage.text, elapsed)

				return nil
			}
		case <-ticker.C:
			w.spinner.Message(w.progressWithRemaining(ctx))
		case <-ctx.Done():
			switch ctx.Err() {
			case context.DeadlineExceeded:
				msg := "timeout waiting for %s"
				w.spinner.StopFailMessage(format.Progressf("", msg, w.kind))
				_ = w.spinner.StopFail()

				return fmt.Errorf(msg, w.kind)
			case context.Canceled:
				_ = w.spinner.StopFail()
				return nil
			}
		}
	}
}

func resourceAvailable(event watch.Event) (bool, error) {
	mg, ok := event.Object.(resource.Managed)
	if !ok {
		return false, nil
	}

	return isAvailable(mg), nil
}

func isAvailable(mg resource.Managed) bool {
	return mg.GetCondition(runtimev1.TypeReady).Reason == runtimev1.ReasonAvailable &&
		mg.GetCondition(runtimev1.TypeReady).Status == corev1.ConditionTrue
}

func getName(name string) string {
	if len(name) != 0 {
		return name
	}

	return codename.Generate(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
}

// stringSlice converts a slice of string like elements to a slice of strings.
func stringSlice[K ~string](elems []K) []string {
	s := make([]string, 0, len(elems))
	for _, elem := range elems {
		s = append(s, string(elem))
	}
	return s
}

// stringerSlice converts a slice of elements implementing [fmt.Stringer] to a slice of strings.
func stringerSlice[T fmt.Stringer](slice []T) []string {
	strings := make([]string, 0, len(slice))
	for _, e := range slice {
		strings = append(strings, e.String())
	}
	return strings
}
