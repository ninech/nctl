package cli

import (
	"slices"
	"strings"

	"github.com/alecthomas/kong"
)

// Name is the name of the CLI binary.
const Name = "nctl"

// Command lines suggested to users in hints and error messages.
const (
	// LoginCmd logs in to the API.
	LoginCmd = Name + " auth login"
	// WhoAmICmd shows the current session.
	WhoAmICmd = Name + " auth whoami"
	// GetProjectsCmd lists the projects.
	GetProjectsCmd = Name + " get projects"
)

// GetCmd returns the command line getting resources of kind.
// The kind is lowercased, so that it can be passed as it is.
func GetCmd(kind string, args ...string) string {
	return strings.Join(append([]string{Name, "get", strings.ToLower(kind)}, args...), " ")
}

// CredentialsPrinter is implemented by get sub-commands
// which can print the credentials of a resource.
type CredentialsPrinter interface {
	// CredentialsFlag returns the flag printing the credentials.
	CredentialsFlag() string
}

// GetCredentialsCmd returns the command line printing the credentials of the resource name in project.
// It uses the get sub-command named or aliased like node, a sub-command of another verb,
// e.g. the one of Kong's selected command of create.
// It returns false if node is nil, there is no such get sub-command
// or it does not implement [CredentialsPrinter].
func GetCredentialsCmd(node *kong.Node, name, project string) (string, bool) {
	if node == nil {
		return "", false
	}
	root := node
	for root.Parent != nil {
		root = root.Parent
	}

	for _, verb := range root.Children {
		if verb.Name != "get" {
			continue
		}
		for _, cmd := range verb.Children {
			if (cmd.Name != node.Name && !slices.Contains(cmd.Aliases, node.Name)) || !cmd.Target.CanAddr() {
				continue
			}
			printer, ok := cmd.Target.Addr().Interface().(CredentialsPrinter)
			if !ok {
				return "", false
			}
			return GetCmd(node.Name, name, "--project", project, printer.CredentialsFlag()), true
		}
	}
	return "", false
}

// SetProjectCmd returns the command line setting the project,
// with a placeholder when project is empty.
func SetProjectCmd(project string) string {
	if project == "" {
		project = "<project-name>"
	}

	return Name + " auth set-project " + project
}
