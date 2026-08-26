package main

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/apifield"
	"github.com/ninech/nctl/internal/apiresource"
	"github.com/ninech/nctl/internal/completion"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKongVars makes sure that the kongVariables function will not run into an
// error. As it is based mostly on static input, a simple test should be enough.
func TestKongVars(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	vars, err := kongVariables()
	is.NoError(err)
	is.NotEmpty(vars)
}

func TestNoAPIClientRequired(t *testing.T) {
	t.Parallel()
	is := assert.New(t)

	// Commands use the resolved format from kong.Context.Command().
	// --help and --version are not tested here because Kong exits
	// during Parse before noAPIClientRequired is called.
	tests := []struct {
		command  string
		expected bool
	}{
		{"auth login", true},
		{"auth login <organization>", true},
		{"auth logout", true},
		{"auth oidc", true},
		{"auth client-credentials", true},
		{"completions", true},
		{"completions bash", true},
		{"get", false},
		{"get application", false},
		{"get application <name>", false},
		{"create application <name>", false},
		{"exec application <name>", false},
		{"", false},
	}
	for _, tt := range tests {
		is.Equal(tt.expected, noAPIClientRequired(tt.command), "command: %q", tt.command)
	}
}

func TestCompletionPredictorsRegistered(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	_, err := completion.Command(t.Context(), newTestParser(t))
	is.NoError(err)
}

func TestAPIFieldFlagCompletion(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	cmd, err := completion.Command(t.Context(), newTestParser(t))
	is.NoError(err)

	versions := apifield.Predictors()["apifield:postgres_version"].Predict(complete.Args{})
	is.NotEmpty(versions, "the postgres_version field knows no values")

	predictor, ok := cmd.Sub["create"].Sub["postgres"].GlobalFlags["--postgres-version"]
	is.True(ok, "no completion registered for the --postgres-version flag")
	is.ElementsMatch(versions, predictor.Predict(complete.Args{}))

	// Fields with only a default still need a predictor registered with kong-completion.
	predictor, ok = cmd.Sub["create"].Sub["keyvaluestore"].GlobalFlags["--memory-size"]
	is.True(ok, "no completion registered for the --memory-size flag")
	is.NotNil(predictor, "a field without values still predicts")
	is.Empty(predictor.Predict(complete.Args{}), "a field without values offers none")
}

// Aliases share the command node and do not need separate verification.
func TestResourceNameCompletionResolves(t *testing.T) {
	t.Parallel()
	is := assert.New(t)

	scheme, err := api.NewScheme()
	require.NoError(t, err)

	parser := newTestParser(t)
	cmd, err := completion.Command(t.Context(), parser)
	require.NoError(t, err)

	var walk func(cmd complete.Command, node *kong.Node, path string)
	walk = func(cmd complete.Command, node *kong.Node, path string) {
		for _, child := range node.Children {
			if child == nil || child.Type != kong.CommandNode {
				continue
			}
			sub, completed := cmd.Sub[child.Name]
			if completesResourceName(child) {
				if !is.True(completed, "%s %s is excluded from completion", path, child.Name) {
					continue
				}
				_, err := apiresource.FindListKind(scheme, apiresource.OfCommand(child))
				is.NoError(err, "%s %s completes no resource", path, child.Name)
			}
			if completed {
				walk(sub, child, path+" "+child.Name)
			}
		}
	}
	walk(cmd, parser.Model.Node, "nctl")
}

func completesResourceName(node *kong.Node) bool {
	for _, positional := range node.Positional {
		if positional.Tag != nil && positional.Tag.Get("completion-predictor") == completion.ResourceName {
			return true
		}
	}

	return false
}
