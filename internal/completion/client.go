package completion

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
)

// apiTimeout limits API requests during shell completion to prevent hanging.
const apiTimeout = 5 * time.Second

const apiClusterFlag = "api-cluster"

// clientFunc lazily returns an API client for completion predictors.
type clientFunc func() (*api.Client, error)

// newClient creates an API client configured for shell completion using a static token.
func newClient(ctx context.Context, apiCluster string) (*api.Client, error) {
	c, err := api.New(ctx, apiCluster, "", api.StaticToken(ctx))
	if err != nil {
		return nil, fmt.Errorf("cannot create an API client for the cluster %q: %w", apiCluster, err)
	}

	return c, nil
}

// apiCluster resolves the cluster context for the command line being completed
// (flag > env > default). This runs before Kong parses flags.
func apiCluster(parser *kong.Kong) string {
	flag := findFlag(parser, apiClusterFlag)
	if flag == nil {
		return ""
	}

	// The command line is only available through the environment, as
	// posener/complete consumes the arguments while matching subcommands.
	if cluster := flagValue(argsFromENV(), flagNames(flag)...); cluster != "" {
		return cluster
	}

	for _, env := range flag.Tag.Envs {
		if cluster := os.Getenv(env); cluster != "" {
			return cluster
		}
	}

	return flag.Default
}
