package format

import (
	"fmt"
	"os"
	"strings"
)

const (
	LoginCommand          = "auth login"
	getApplicationCommand = "get application"
)

type command string

// Command can be used to print how certain nctl commands can be executed
func Command() command {
	return command(os.Args[0])
}

// Login returns the login command
func (c command) Login() string {
	return fmt.Sprintf("%s %s", string(c), LoginCommand)
}

// GetApplication returns the command for getting applications with nctl
func (c command) GetApplication(extraFields ...string) string {
	if len(extraFields) == 0 {
		return fmt.Sprintf("%s %s", string(c), getApplicationCommand)
	}
	return fmt.Sprintf("%s %s %s", string(c), getApplicationCommand, strings.Join(extraFields, " "))
}
