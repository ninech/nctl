// Package main is the entry point for nctl.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	dockerterm "github.com/moby/term"
	"github.com/ninech/nctl/internal/cmd"
)

// Set by the release build with -ldflags.
var (
	version string
	commit  string
	date    string
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Restore the default handling after the first signal,
	// so that a second one terminates nctl even if the command does not stop.
	context.AfterFunc(ctx, stop)

	// On Windows, StdStreams enables the virtual terminal sequences of the console,
	// or emulates them where the console does not support them.
	stdin, stdout, stderr := dockerterm.StdStreams()
	code := cmd.Run(ctx, os.Args[1:], stdin, stdout, stderr, versionOutput(version, commit, date))
	stop()
	os.Exit(code)
}

func versionOutput(version, commit, date string) string {
	var extra []string

	if commit != "" {
		extra = append(extra, "commit: "+commit)
	}
	if date != "" {
		extra = append(extra, "date: "+date)
	}
	if len(extra) > 0 {
		return fmt.Sprintf("%s (%s)", version, strings.Join(extra, ", "))
	}
	return version
}

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	if version == "" {
		version = info.Main.Version
	}

	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			commit = kv.Value
		case "vcs.time":
			date = kv.Value
		}
	}
}
