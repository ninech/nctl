package main

import (
	"errors"
	"go/build"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
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

const (
	// module is the import path of this module.
	module = "github.com/ninech/nctl"
	// verbDir is the directory below the module root holding the verb packages.
	verbDir = "internal/cmd"
)

// verbPackages are the packages implementing a CLI verb, by name below verbDir.
// They must not import each other:
// anything two verbs share belongs in a resource package below them.
var verbPackages = []string{"apply", "auth", "copy", "create", "delete", "edit", "exec", "get", "logs", "update"}

// verbImport is a direct import of the verb package to by the verb package
// from, including from its tests.
type verbImport struct{ from, to string }

// allowedVerbImports are the verb-to-verb imports that still exist. Each
// entry must be removed by the PR that breaks that edge; the test fails once
// an entry is stale.
var allowedVerbImports = map[verbImport]bool{}

func TestVerbsDoNotImportEachOther(t *testing.T) {
	t.Parallel()
	is := assert.New(t)

	verbImportPrefix := path.Join(module, verbDir) + "/"
	found := map[verbImport]bool{}
	for _, from := range verbPackages {
		pkg, err := build.Default.ImportDir(filepath.Join(verbDir, from), 0)
		require.NoError(t, err)
		for _, imported := range slices.Concat(pkg.Imports, pkg.TestImports, pkg.XTestImports) {
			if to, ok := strings.CutPrefix(imported, verbImportPrefix); ok && slices.Contains(verbPackages, to) {
				found[verbImport{from, to}] = true
			}
		}
	}

	for edge := range found {
		is.True(allowedVerbImports[edge], "verb %s imports verb %s: move what they share to a resource package", edge.from, edge.to)
	}
	for edge := range allowedVerbImports {
		is.True(found[edge], "verb %s no longer imports verb %s: remove the edge from allowedVerbImports", edge.from, edge.to)
	}
}

// leafPackages use nothing else of the module:
// cli holds the error and exit code vocabulary,
// format and logbox only render output.
var leafPackages = []string{"internal/cli", "internal/format", "internal/logbox"}

// TestLayering guards the layers below the verbs.
// Non-test imports are checked strictly;
// tests may additionally import internal/testutil for fixtures.
// See layeringViolation for the rules.
func TestLayering(t *testing.T) {
	t.Parallel()
	is := assert.New(t)

	packages := modulePackages(t)
	for _, dir := range slices.Concat([]string{".", "api"}, leafPackages) {
		require.Contains(t, packages, dir, "the module walk misses %s", dir)
	}

	for from, pkg := range packages {
		check := func(imports []string, fixtures bool) {
			for _, imported := range imports {
				to, ok := strings.CutPrefix(imported, module+"/")
				if !ok || (fixtures && to == "internal/testutil") {
					continue
				}
				if reason := layeringViolation(from, to); reason != "" {
					is.Fail("layering violation", "%s imports %s, but %s", from, to, reason)
				}
			}
		}
		check(pkg.Imports, false)
		check(slices.Concat(pkg.TestImports, pkg.XTestImports), true)
	}
}

// layeringViolation tells why the package in directory from must not import the package in directory to,
// or "" when the import is fine.
// Both are below the module root, "." being the main package.
func layeringViolation(from, to string) string {
	switch {
	case slices.Contains(leafPackages, from):
		return "leaf packages use nothing else of the module"
	case within(from, "api") && !within(to, "api") && to != "internal/cli":
		return "the api packages only use api/... and internal/cli"
	case within(to, verbDir) && from != "." && !within(from, verbDir):
		return "verbs are only imported by main and by other verbs"
	}

	return ""
}

// within reports whether dir is root or lies below it.
func within(dir, root string) bool {
	return dir == root || strings.HasPrefix(dir, root+"/")
}

// modulePackages returns the packages of the module by their directory below the module root,
// "." being the main package.
func modulePackages(t *testing.T) map[string]*build.Package {
	t.Helper()

	packages := map[string]*build.Package{}
	err := filepath.WalkDir(".", func(dir string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}
		// Skip what the go tool skips,
		// plus the generated completions and build output.
		if name := entry.Name(); dir != "." && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
			slices.Contains([]string{"testdata", "completions", "dist"}, name)) {
			return filepath.SkipDir
		}
		pkg, err := build.Default.ImportDir(dir, 0)
		if _, ok := errors.AsType[*build.NoGoError](err); ok {
			return nil
		}
		if err != nil {
			return err
		}
		packages[filepath.ToSlash(dir)] = pkg

		return nil
	})
	require.NoError(t, err)

	return packages
}
