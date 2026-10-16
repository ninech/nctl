// Package create provides functionality to create resources in the nine.ch API.
package create

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/lucasepe/codename"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/waiter"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type Cmd struct {
	FromFile            fromFile             `cmd:"" group:"create-general" default:"withargs" name:"-f <file>" help:"Create any resource from a yaml or json file."`
	VCluster            vclusterCmd          `cmd:"" group:"create-infra" name:"vcluster" help:"Create a new vcluster."`
	APIServiceAccount   apiServiceAccountCmd `cmd:"" group:"create-access" name:"apiserviceaccount" aliases:"asa" help:"Create a new API Service Account."`
	Project             projectCmd           `cmd:"" group:"create-access" name:"project" help:"Create a new project."`
	Config              configCmd            `cmd:"" group:"create-apps" name:"config"  help:"Create a new deplo.io Project Configuration."`
	Application         applicationCmd       `cmd:"" group:"create-apps" name:"application" aliases:"app,application" help:"Create a new deplo.io Application."`
	MySQL               mySQLCmd             `cmd:"" group:"create-storage" name:"mysql" help:"Create a new MySQL instance."`
	MySQLDatabase       mysqlDatabaseCmd     `cmd:"" group:"create-storage" name:"mysqldatabase" help:"Create a new MySQL database."`
	Postgres            postgresCmd          `cmd:"" group:"create-storage" name:"postgres" help:"Create a new PostgreSQL instance."`
	PostgresDatabase    postgresDatabaseCmd  `cmd:"" group:"create-storage" name:"postgresdatabase" help:"Create a new PostgreSQL database."`
	KeyValueStore       keyValueStoreCmd     `cmd:"" group:"create-storage" name:"keyvaluestore" aliases:"kvs" help:"Create a new KeyValueStore instance."`
	OpenSearch          openSearchCmd        `cmd:"" group:"create-storage" name:"opensearch" aliases:"os" help:"Create a new OpenSearch cluster."`
	CloudVirtualMachine cloudVMCmd           `cmd:"" group:"create-infra" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Create a new CloudVM."`
	ServiceConnection   serviceConnectionCmd `cmd:"" group:"create-network" name:"serviceconnection" aliases:"sc" help:"Create a new ServiceConnection."`
	StaticEgress        staticEgressCmd      `cmd:"" group:"create-network" name:"staticegress" aliases:"se" help:"Create a new StaticEgress."`
	Bucket              bucketCmd            `cmd:"" group:"create-storage" name:"bucket" help:"Create a new Bucket."`
	BucketUser          bucketUserCmd        `cmd:"" group:"create-storage" name:"bucketuser" aliases:"bu" help:"Create a new BucketUser."`
	Grafana             grafanaCmd           `cmd:"" group:"create-observability" name:"grafana" help:"Create a new Grafana instance."`
}

// ResourceCmd is the shared base for the create sub-commands.
//
// It has to be exported so that Kong initializes the embedded
// [format.Writer], see [format.Writer.BeforeApply].
type ResourceCmd struct {
	format.Writer `kong:"-"`
	Name          string        `arg:"" help:"Name of the new resource. A random name is generated if omitted." default:""`
	Wait          bool          `default:"true" help:"Wait until resource is fully created."`
	WaitTimeout   time.Duration `default:"30m" help:"Duration to wait for resource getting ready. Only relevant if wait is set."`
}

// BeforeApply initializes Writer from the bound [format.IOStreams].
func (cmd *ResourceCmd) BeforeApply(streams format.IOStreams) error {
	return cmd.Writer.BeforeApply(streams)
}

type creator struct {
	format.Writer

	client *api.Client
	mg     resource.Managed
	kind   string

	timeout time.Duration
}

func (cmd *ResourceCmd) newCreator(client *api.Client, mg resource.Managed, kind string) *creator {
	return &creator{client: client, mg: mg, kind: kind, timeout: cmd.WaitTimeout, Writer: cmd.Writer}
}

func (c *creator) createResource(ctx context.Context) error {
	if err := c.client.Create(ctx, c.mg); err != nil {
		return fmt.Errorf("unable to create %s %q: %w", c.kind, c.mg.GetName(), err)
	}

	c.Successf("🏗", "created %s %q in project %q", c.kind, c.mg.GetName(), c.mg.GetNamespace())
	return nil
}

// createResourceInLocation creates the resource and retries once in another
// location if the API server rejects the one it has. requested is the location
// the user asked for, setLocation applies the fallback.
func (c *creator) createResourceInLocation(
	ctx context.Context,
	requested meta.LocationName,
	setLocation func(meta.LocationName),
) error {
	err := c.createResource(ctx)
	if err == nil {
		return nil
	}

	// the user picked the location and it cannot be changed afterwards, so
	// never create the resource somewhere else.
	if requested != "" {
		return err
	}

	// the API server returns them sorted, so the fallback is stable.
	locations := availableLocations(err)
	if len(locations) == 0 {
		return err
	}
	fallback := locations[0]

	c.Warningf(
		"the default location does not currently accept new %s resources, creating in %q instead. "+
			"The location cannot be changed later, pass --location to choose a different one.",
		c.kind, fallback,
	)
	setLocation(fallback)

	return c.createResource(ctx)
}

func (c *creator) wait(ctx context.Context, stages ...waiter.Stage) error {
	w := &waiter.Waiter{Writer: c.Writer, Client: c.client, Kind: c.kind, Object: c.mg}
	return w.Wait(ctx, stages...)
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
