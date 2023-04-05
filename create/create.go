package create

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/briandowns/spinner"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/lucasepe/codename"
	"github.com/mattn/go-isatty"
	"github.com/ninech/nctl/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Cmd struct {
	VCluster          vclusterCmd          `cmd:"" name:"vcluster" help:"Create a new vcluster."`
	APIServiceAccount apiServiceAccountCmd `cmd:"" name:"apiserviceaccount" aliases:"asa" help:"Create a new API Service Account."`
}

// resultFunc is the function called on a watch event during creation. It
// should return true whenever the wait can be considered done.
type resultFunc func(watch.Event) (bool, error)

type creator struct {
	kind       string
	mg         resource.Managed
	objectList runtimeclient.ObjectList
}

func newCreator(mg resource.Managed, resourceName string, objectList runtimeclient.ObjectList) *creator {
	return &creator{mg: mg, kind: resourceName, objectList: objectList}
}

func (c *creator) createResource(ctx context.Context, client *api.Client) error {
	if err := client.Create(ctx, c.mg); err != nil {
		return fmt.Errorf("unable to create %s %q: %w", c.kind, c.mg.GetName(), err)
	}

	fmt.Printf(" ✓ created %s %q\n", c.kind, c.mg.GetName())
	return nil
}

func (w *creator) wait(ctx context.Context, client *api.Client, onResult resultFunc) error {
	fmt.Printf(" ✓ waiting for %s to be ready ⏳\n", w.kind)
	spin := spinner.New(spinner.CharSets[7], 100*time.Millisecond)
	spin.Prefix = " "
	if isatty.IsTerminal(os.Stdout.Fd()) {
		spin.Start()
	}
	defer spin.Stop()

	watch, err := client.Watch(
		ctx, w.objectList,
		runtimeclient.InNamespace(w.mg.GetNamespace()),
		runtimeclient.MatchingFields{"metadata.name": w.mg.GetName()},
	)
	if err != nil {
		return fmt.Errorf("unable to watch %s: %w", w.kind, err)
	}

	for {
		select {
		case res := <-watch.ResultChan():
			done, err := onResult(res)
			if err != nil {
				return err
			}

			if done {
				watch.Stop()
				spin.Stop()
				fmt.Printf(" ✓ %s ready 🐧\n", w.kind)
				return nil
			}
		case <-ctx.Done():
			spin.Stop()
			return fmt.Errorf("timeout waiting for %s", w.kind)
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
