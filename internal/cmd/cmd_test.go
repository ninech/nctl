package cmd

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/apifield"
	"github.com/ninech/nctl/internal/apiresource"
	"github.com/ninech/nctl/internal/cli"
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

	vars, err := kongVariables("test")
	is.NoError(err)
	is.NotEmpty(vars)
}

// TestCommandsWithoutAPIClient lists the commands which run without an API client.
// Kong only builds the client for commands whose Run method takes one,
// so these have to work without the user being logged in.
func TestCommandsWithoutAPIClient(t *testing.T) {
	t.Parallel()

	var withoutClient []string
	for _, path := range commandPaths(newTestParser(t).Model.Node, nil) {
		node := nodeAt(newTestParser(t).Model.Node, path)
		run := node.Target.Addr().MethodByName("Run")
		require.True(t, run.IsValid(), "command %q has no Run method", path)
		if !slices.Contains(params(run.Type()), reflect.TypeFor[*api.Client]()) {
			withoutClient = append(withoutClient, strings.Join(path, " "))
		}
	}

	assert.ElementsMatch(t, []string{
		"auth login",
		"auth logout",
		"auth oidc",
		"auth client-credentials",
		"completions",
	}, withoutClient)
}

// params returns the parameter types of the function type fn.
func params(fn reflect.Type) []reflect.Type {
	var types []reflect.Type
	for i := range fn.NumIn() {
		types = append(types, fn.In(i))
	}

	return types
}

func TestCompletionPredictorsRegistered(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	_, err := completion.Command(t.Context(), newTestParser(t), slog.New(slog.DiscardHandler))
	is.NoError(err)
}

func TestAPIFieldFlagCompletion(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	cmd, err := completion.Command(t.Context(), newTestParser(t), slog.New(slog.DiscardHandler))
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
	cmd, err := completion.Command(t.Context(), parser, slog.New(slog.DiscardHandler))
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

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "version",
			args:       []string{"--version"},
			wantCode:   cli.ExitOK,
			wantStdout: "test-version",
		},
		{
			name:       "help",
			args:       []string{"--help"},
			wantCode:   cli.ExitOK,
			wantStdout: "Usage: nctl <command>",
		},
		{
			name:       "missing command",
			args:       []string{"get"},
			wantCode:   cli.ExitError,
			wantStdout: "Usage: nctl get",
			wantStderr: `💡 Your command: "nctl get"`,
		},
		{
			name:       "unknown flag",
			args:       []string{"--bogus"},
			wantCode:   cli.ExitUsageError,
			wantStderr: "unknown flag --bogus",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)

			var stdout, stderr bytes.Buffer
			code := Run(t.Context(), tt.args, &bytes.Buffer{}, &stdout, &stderr, "test-version")

			is.Equal(tt.wantCode, code)
			is.Contains(stdout.String(), tt.wantStdout)
			is.Contains(stderr.String(), tt.wantStderr)
		})
	}
}

func TestRunWithoutLogin(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	is := assert.New(t)

	var stdout, stderr bytes.Buffer
	code := Run(t.Context(), []string{"get", "applications"}, &bytes.Buffer{}, &stdout, &stderr, "test-version")

	is.Equal(cli.ExitError, code)
	is.Empty(stdout.String())
	is.Contains(stderr.String(), cli.LoginCmd)
}
