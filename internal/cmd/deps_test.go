package cmd_test

import (
	"errors"
	"go/build"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// module is the import path of this module.
	module = "github.com/ninech/nctl"
	// verbDir is the directory below the module root
	// holding the root command and, below it, the verb packages.
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
		pkg, err := build.Default.ImportDir(filepath.Join(moduleRoot(t), verbDir, from), 0)
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
	case to != verbDir && within(to, verbDir) && !within(from, verbDir):
		return "verbs are only imported by the root command and by other verbs"
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

	root := moduleRoot(t)
	packages := map[string]*build.Package{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}
		dir, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Skip what the go tool skips,
		// plus the generated completions and build output.
		if name := entry.Name(); dir != "." && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
			slices.Contains([]string{"testdata", "completions", "dist"}, name)) {
			return filepath.SkipDir
		}
		pkg, err := build.Default.ImportDir(path, 0)
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

// moduleRoot returns the directory holding the go.mod of the module.
func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "no go.mod found above the working directory")
		dir = parent
	}
}
