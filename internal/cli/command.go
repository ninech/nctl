package cli

import (
	"strings"
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

// SetProjectCmd returns the command line setting the project,
// with a placeholder when project is empty.
func SetProjectCmd(project string) string {
	if project == "" {
		project = "<project-name>"
	}

	return Name + " auth set-project " + project
}
