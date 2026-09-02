// Package completion configures shell completion predictors for the CLI.
package completion

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/alecthomas/kong"
	kongcompletion "github.com/jotaen/kong-completion"
	management "github.com/ninech/apis/management/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/apifield"
	"github.com/posener/complete"
)

// Cmd generates shell completion scripts.
type Cmd = kongcompletion.Completion

const localFile = "local:file"

const (
	ResourceName      = "client:resource_name"
	ProjectName       = "client:project_name"
	PostgresDatabases = "client:postgres_databases"
	MySQLDatabases    = "client:mysql_databases"
)

// apiPredictors builds the predictors that query the API. It shares a single
// lazily created client between them, so that completing a command line
// authenticates at most once.
type apiPredictors struct {
	client  clientFunc
	project projectFinder
}

func newAPIPredictors(ctx context.Context, parser *kong.Kong) *apiPredictors {
	return &apiPredictors{
		client: sync.OnceValues(func() (*api.Client, error) {
			return newClient(ctx, apiCluster(parser))
		}),
		project: newProjectFinder(parser),
	}
}

// options returns all kong-completion options. The [ResourceName] predictor is
// only a placeholder here, as the resource it completes depends on the command
// it is registered for, see [bindResourceNames].
func (p *apiPredictors) options() []kongcompletion.Option {
	return []kongcompletion.Option{
		kongcompletion.WithPredictor(localFile, complete.PredictFiles("*")),
		kongcompletion.WithPredictors(apifield.Predictors()),
		kongcompletion.WithPredictors(map[string]complete.Predictor{
			ResourceName: unboundResourceName{},
			ProjectName: newResourceNameWithKind(p.client, p.project,
				management.SchemeGroupVersion.WithKind(reflect.TypeFor[management.ProjectList]().Name())),
			PostgresDatabases: newInstanceDatabases(p.client, p.project, storage.PostgresGroupVersionKind),
			MySQLDatabases:    newInstanceDatabases(p.client, p.project, storage.MySQLGroupVersionKind),
		}),
	}
}

func (p *apiPredictors) resourceName(resource string) complete.Predictor {
	return newResourceName(p.client, p.project, resource)
}

// Command builds the complete.Command tree for shell completion.
func Command(ctx context.Context, parser *kong.Kong) (complete.Command, error) {
	predictors := newAPIPredictors(ctx, parser)

	cmd, err := kongcompletion.Command(parser, predictors.options()...)
	if err != nil {
		return complete.Command{}, err
	}

	if parser == nil || parser.Model == nil {
		return cmd, nil
	}
	if err := bindResourceNames(cmd, parser.Model.Node, predictors.resourceName); err != nil {
		return complete.Command{}, err
	}
	if err := bindArgFlags(cmd, parser.Model.Node); err != nil {
		return complete.Command{}, err
	}

	return cmd, nil
}

// Register configures the Kong parser to intercept shell completion requests.
func Register(ctx context.Context, parser *kong.Kong) error {
	// Verify required flags exist; missing flags would cause completion
	// to silently fall back to kubeconfig defaults.
	for _, name := range []string{apiClusterFlag, projectFlag} {
		if findFlag(parser, name) == nil {
			return fmt.Errorf("error on shell completion: the CLI defines no %q flag", name)
		}
	}

	// Build the tree manually to retain bound resource predictors and catch
	// misconfigurations before exiting.
	cmd, err := Command(ctx, parser)
	if err != nil {
		return fmt.Errorf("error on shell completion: %w", err)
	}

	completer := complete.New(parser.Model.Name, cmd)
	completer.Out = parser.Stdout
	if completer.Complete() {
		parser.Exit(0)
	}

	return nil
}
