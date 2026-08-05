package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/internal/format"
	"github.com/stretchr/testify/require"
)

// TestKongInitializesIO parses every command of the CLI and asserts that Kong
// initialized every [format.Writer] and [format.Reader] the selected command
// reaches through embedded structs.
//
// Kong only descends into exported embedded fields while looking for hook
// methods, so a command whose writer sits behind an unexported base struct
// never receives one.
func TestKongInitializesIO(t *testing.T) {
	t.Parallel()

	model := newTestParser(t).Model
	file := writeTempFile(t)

	for _, path := range commandPaths(model.Node, nil) {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			parser := newTestParser(t)
			node := nodeAt(parser.Model.Node, path)
			args := commandArgs(node, path, file)

			kctx, err := parser.Parse(args)
			is.NoError(err, "args: %q", args)

			// Guard against a synthesized command line that parses but
			// selects a different command than the one under test.
			is.Equal(path, selectedPath(kctx.Selected()), "args: %q", args)

			var uninitialized []string
			for n := kctx.Selected(); n != nil; n = n.Parent {
				uninitialized = append(uninitialized, uninitializedIO(n.Target, n.Name)...)
			}
			is.Empty(uninitialized, "Kong did not initialize all writers and readers")
		})
	}
}

// newTestParser builds the parser of the CLI with the IO bound to a buffer.
func newTestParser(t *testing.T) *kong.Kong {
	t.Helper()

	parser, err := newParser(t.Context(), &rootCommand{}, &bytes.Buffer{}, &bytes.Buffer{})
	require.NoError(t, err)

	return parser
}

// writeTempFile returns the path of a file holding an empty resource, for the
// commands which read resources from a file.
func writeTempFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o600))

	return path
}

// commandPaths returns the node names of all commands which can be run.
func commandPaths(node *kong.Node, path []string) [][]string {
	if len(node.Children) == 0 {
		return [][]string{path}
	}

	var paths [][]string
	for _, child := range node.Children {
		paths = append(paths, commandPaths(child, append(slices.Clone(path), child.Name))...)
	}

	return paths
}

// nodeAt returns the node the given command path points to.
func nodeAt(node *kong.Node, path []string) *kong.Node {
	for _, name := range path {
		for _, child := range node.Children {
			if child.Name == name {
				node = child
				break
			}
		}
	}

	return node
}

// selectedPath returns the command path Kong resolved the arguments to.
func selectedPath(node *kong.Node) []string {
	var path []string
	for ; node != nil && node.Type != kong.ApplicationNode; node = node.Parent {
		path = append([]string{node.Name}, path...)
	}

	return path
}

// flagValues holds values for flag types which cannot be satisfied by an
// arbitrary string, keyed by the type of the flag target.
var flagValues = map[string]string{
	"application.TypedReference": "application/test-value",
}

// commandArgs builds a command line which selects node. Only the arguments and
// flags Kong requires are set, as the commands are never run.
func commandArgs(node *kong.Node, path []string, file string) []string {
	var args []string
	for _, name := range path {
		// Default commands are named after the flag which selects them
		// (`-f <file>`) and are not passed as an argument themselves.
		if !strings.HasPrefix(name, "-") {
			args = append(args, name)
		}
	}

	for _, arg := range node.Positional {
		if arg.Required {
			args = append(args, "test-value")
		}
	}

	// Flags of a mutually exclusive group are all marked as required, while
	// only one of them may be set.
	exclusive := map[string]bool{}
	for _, flag := range node.Flags {
		// A file flag needs to point to an existing file and selects the
		// default command of the commands reading from a file.
		if flag.Target.Type() == reflect.TypeFor[*os.File]() {
			args = append(args, "--"+flag.Name, file)
			continue
		}
		if !flag.Required || isExclusive(flag.Xor, exclusive) {
			continue
		}
		for _, group := range flag.Xor {
			exclusive[group] = true
		}

		value := "test-value"
		if flag.Enum != "" {
			value, _, _ = strings.Cut(flag.Enum, ",")
		}
		if override, ok := flagValues[flag.Target.Type().String()]; ok {
			value = override
		}
		args = append(args, "--"+flag.Name+"="+value)
	}

	return args
}

// isExclusive reports whether one of the groups was already used.
func isExclusive(groups []string, used map[string]bool) bool {
	for _, group := range groups {
		if used[group] {
			return true
		}
	}

	return false
}

// uninitializedIO returns the field paths of all [format.Writer] and
// [format.Reader] fields which are reachable from target through embedded
// structs and which do not wrap an [io.Writer] or [io.Reader].
func uninitializedIO(target reflect.Value, path string) []string {
	if target.Kind() == reflect.Pointer {
		if target.IsNil() {
			return nil
		}
		target = target.Elem()
	}
	if target.Kind() != reflect.Struct {
		return nil
	}

	var uninitialized []string
	structType := target.Type()
	for i := range target.NumField() {
		field := structType.Field(i)
		fieldPath := path + "." + field.Name

		switch field.Type {
		case reflect.TypeFor[format.Writer](), reflect.TypeFor[format.Reader]():
			// Both wrap their interface in the first field.
			if target.Field(i).Field(0).IsNil() {
				uninitialized = append(uninitialized, fieldPath)
			}
		default:
			// Only embedded structs are followed, as any other field
			// belongs to a different command with its own hooks.
			if field.Anonymous {
				uninitialized = append(uninitialized, uninitializedIO(target.Field(i), fieldPath)...)
			}
		}
	}

	return uninitialized
}
