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

// GetCmd returns the command line getting a resource.
// The arguments are lowercased,
// so that kinds can be passed as they are.
func GetCmd(args ...string) string {
	lower := make([]string, 0, len(args))
	for _, arg := range args {
		lower = append(lower, strings.ToLower(arg))
	}

	return Name + " get " + strings.Join(lower, " ")
}

// SetProjectCmd returns the command line setting the project,
// with a placeholder when project is empty.
func SetProjectCmd(project string) string {
	if project == "" {
		project = "<project-name>"
	}

	return Name + " auth set-project " + project
}
