package exec

import (
	"context"
	"fmt"
	"io"

	dockerterm "github.com/moby/term"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/term"
)

const (
	appBuildTypeBuildpack  appBuildType = "buildpack"
	appBuildTypeDockerfile appBuildType = "dockerfile"
	buildpackEntrypoint                 = "launcher"
	defaultShellBuildpack               = "/bin/bash"
	defaultShellDockerfile              = "/bin/sh"
)

// appBuildType describes the way how the app was build (buildpack/dockerfile)
type appBuildType string

type remoteCommandParameters struct {
	replicaName      string
	replicaNamespace string
	command          []string
	tty              bool
	enableStdin      bool
	stdin            io.Reader
	stdout           io.Writer
	stderr           io.Writer
	restConfig       *rest.Config
}

type applicationCmd struct {
	resourceCmd
	Stdin   bool     `name:"stdin" short:"i" help:"Pass stdin to the application" default:"true"`
	Tty     bool     `name:"tty" short:"t" help:"Stdin is a TTY" default:"true"`
	Command []string `arg:"" help:"command to execute" optional:""`
}

// Help displays examples for the application exec command
func (ac applicationCmd) Help() string {
	return `Examples:
  # Open a shell in a buildpack/dockerfile built application. The dockerfile
  # built application needs a valid "/bin/sh" shell to be installed.
  nctl exec app myapp

  # Get output from running the 'date' command in an application replica.
  nctl exec app myapp -- date

  # Use redirection to execute a command.
  echo date | nctl exec app myapp

  # In certain situations it might be needed to not redirect stdin. This can be
  # achieved by using the "stdin" flag:
  nctl exec app --stdin=false myapp -- <command>
  `
}

func (cmd *applicationCmd) Run(ctx context.Context, client *api.Client, exec *Cmd) error {
	replicaName, buildType, err := cmd.getReplica(ctx, client)
	if err != nil {
		return fmt.Errorf("error when searching for replica to connect: %w", err)
	}
	config, err := client.DeploioRuntimeConfig(ctx)
	if err != nil {
		return fmt.Errorf("can not create deplo.io cluster rest config: %w", err)
	}
	// use dockerterm to gather the std io streams (windows supported)
	stdin, stdout, stderr := dockerterm.StdStreams()
	return executeRemoteCommand(
		ctx,
		remoteCommandParameters{
			replicaName:      replicaName,
			replicaNamespace: client.Project,
			command:          replicaCommand(buildType, cmd.Command),
			tty:              cmd.Tty,
			enableStdin:      cmd.Stdin,
			stdin:            stdin,
			stdout:           stdout,
			stderr:           stderr,
			restConfig:       config,
		})
}

// getReplica finds a replica of the latest available release
func (cmd *applicationCmd) getReplica(ctx context.Context, client *api.Client) (string, appBuildType, error) {
	release, err := util.ApplicationLatestAvailableRelease(ctx, client, client.Name(cmd.Name))
	if err != nil {
		return "", "", err
	}
	buildType := appBuildTypeBuildpack
	if release.Spec.ForProvider.DockerfileBuild {
		buildType = appBuildTypeDockerfile
	}
	if len(release.Status.AtProvider.ReplicaObservation) == 0 {
		return "", buildType, fmt.Errorf("no replica information found for release %s", release.Name)
	}
	if replica := readyReplica(release.Status.AtProvider.ReplicaObservation); replica != "" {
		return replica, buildType, nil
	}
	return "", buildType, fmt.Errorf("no ready replica found for release %s", release.Name)
}

func readyReplica(replicaObs []apps.ReplicaObservation) string {
	for _, obs := range replicaObs {
		if obs.Status == apps.ReplicaStatusReady {
			return obs.ReplicaName
		}
	}
	return ""
}

// setupTTY sets up a TTY for command execution
func setupTTY(params *remoteCommandParameters) term.TTY {
	t := term.TTY{
		Out: params.stdout,
	}
	if !params.enableStdin {
		return t
	}
	t.In = params.stdin
	if !params.tty {
		return t
	}
	if !t.IsTerminalIn() {
		// if this is not a suitable TTY, we don't request one in the
		// exec call and don't set the terminal into RAW mode either
		params.tty = false
		return t
	}
	// if we get to here, the user wants to attach stdin, wants a TTY, and
	// os.Stdin is a terminal, so we can safely set t.Raw to true
	t.Raw = true
	return t
}

func executeRemoteCommand(ctx context.Context, params remoteCommandParameters) error {
	coreClient, err := kubernetes.NewForConfig(params.restConfig)
	if err != nil {
		return err
	}

	tty := setupTTY(&params)
	var sizeQueue remotecommand.TerminalSizeQueue
	if tty.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = tty.MonitorSize(tty.GetSize())

		// unset stderr if it was previously set because both stdout
		// and stderr go over params.stdout when tty is
		// true
		params.stderr = nil
	}
	fn := func() error {
		request := coreClient.CoreV1().RESTClient().
			Post().
			Namespace(params.replicaNamespace).
			Resource("pods").
			Name(params.replicaName).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Command: params.command,
				Stdin:   params.enableStdin,
				Stdout:  params.stdout != nil,
				Stderr:  params.stderr != nil,
				TTY:     params.tty,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(params.restConfig, "POST", request.URL())
		if err != nil {
			return err
		}
		return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:             tty.In,
			Stdout:            params.stdout,
			Stderr:            params.stderr,
			Tty:               params.tty,
			TerminalSizeQueue: sizeQueue,
		})

	}
	return tty.Safe(fn)
}

func replicaCommand(buildType appBuildType, command []string) []string {
	switch buildType {
	case appBuildTypeBuildpack:
		execute := append([]string{buildpackEntrypoint}, command...)
		if len(command) == 0 {
			execute = []string{buildpackEntrypoint, defaultShellBuildpack}
		}
		return execute
	case appBuildTypeDockerfile:
		if len(command) == 0 {
			return []string{defaultShellDockerfile}
		}
		return command
	default:
		return command
	}
}
