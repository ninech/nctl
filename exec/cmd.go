package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/mattn/go-isatty"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/get"
	"github.com/ninech/nctl/internal/cli"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/ipcheck"
)

// cmdExecutor encapsulates resource-specific logic for connecting via an external CLI.
type cmdExecutor[T resource.Managed] interface {
	// Command returns the CLI binary name (e.g. "psql", "mysql", "redis-cli").
	Command() string

	// Endpoint returns "host:port" for the TCP connectivity check.
	Endpoint(res T) string

	// Args builds CLI arguments from the resource and credentials.
	// The returned cleanup func removes any temp files created (e.g. CA cert).
	Args(res T, user, pw string) (args []string, cleanup func(), err error)
}

// accessManager extends cmdExecutor for resources that have access restrictions.
type accessManager[T resource.Managed] interface {
	// AllowedCIDRs returns the current list of allowed CIDRs for the resource.
	AllowedCIDRs(res T) []meta.IPv4CIDR

	// Update patches the resource to allow the given CIDRs.
	Update(ctx context.Context, client *api.Client, res T, cidrs []meta.IPv4CIDR) error
}

// serviceCmd is the shared base for all database exec sub-commands.
type serviceCmd struct {
	resourceCmd
	format.Writer `kong:"-"`
	format.Reader `kong:"-"`
	AllowedCidrs  *[]meta.IPv4CIDR `placeholder:"203.0.113.1/32" help:"Specifies the IP addresses allowed to connect to the instance. Overrides auto-detected public IP."`
	WaitTimeout   time.Duration    `default:"3m" help:"Timeout waiting for connectivity."`
	ExtraArgs     []string         `arg:"" optional:"" passthrough:"" help:"Additional flags passed to the CLI (after --)."`

	// Internal dependencies — nil means use production default.
	runCommand          func(ctx context.Context, name string, args []string) error                                   `kong:"-"`
	lookPath            func(file string) (string, error)                                                             `kong:"-"`
	waitForConnectivity func(ctx context.Context, writer format.Writer, endpoint string, timeout time.Duration) error `kong:"-"`
	openTTYForConfirm   func() (io.ReadCloser, error)                                                                 `kong:"-"`
}

// BeforeApply initializes Writer and Reader from Kong's bound io.Writer and io.Reader.
func (cmd *serviceCmd) BeforeApply(writer io.Writer, reader io.Reader) error {
	return errors.Join(
		cmd.Writer.BeforeApply(writer),
		cmd.Reader.BeforeApply(reader),
	)
}

func (cmd serviceCmd) getRunCommand() func(context.Context, string, []string) error {
	if cmd.runCommand != nil {
		return cmd.runCommand
	}

	return func(ctx context.Context, name string, args []string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
}

func (cmd serviceCmd) getLookPath() func(string) (string, error) {
	if cmd.lookPath != nil {
		return cmd.lookPath
	}

	return exec.LookPath
}

func (cmd serviceCmd) connectivityCheck() func(context.Context, format.Writer, string, time.Duration) error {
	if cmd.waitForConnectivity != nil {
		return cmd.waitForConnectivity
	}

	return waitForConnectivity
}

// openTTY returns the openTTY function to use for confirming prompts.
func (cmd serviceCmd) openTTY() func() (io.ReadCloser, error) {
	if cmd.openTTYForConfirm != nil {
		return cmd.openTTYForConfirm
	}

	return func() (io.ReadCloser, error) {
		return os.Open("/dev/tty")
	}
}

// connectAndExec is the main orchestration function for exec commands.
// It handles path checking, connectivity waiting, and credential retrieval.
func connectAndExec[T resource.Managed](
	ctx context.Context,
	client *api.Client,
	res T,
	connector cmdExecutor[T],
	opts serviceCmd,
) error {
	if err := opts.checkPath(connector.Command()); err != nil {
		return err
	}

	endpoint := connector.Endpoint(res)
	if endpoint == "" {
		return fmt.Errorf("resource %q is not ready yet (no endpoint available)", res.GetName())
	}

	if !quickDial(ctx, endpoint) {
		if am, ok := connector.(accessManager[T]); ok {
			if err := ensureAccess(ctx, client, am, res, opts); err != nil {
				return err
			}
		}

		if err := opts.connectivityCheck()(ctx, opts.Writer, endpoint, opts.WaitTimeout); err != nil {
			return err
		}
	}

	user, pw, err := getCredentials(ctx, client, res)
	if err != nil {
		return err
	}

	args, cleanup, err := connector.Args(res, user, pw)
	if err != nil {
		return fmt.Errorf("building CLI arguments: %w", err)
	}
	defer cleanup()

	args = append(args, opts.ExtraArgs...)

	if err := opts.getRunCommand()(ctx, connector.Command(), args); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return cli.ErrorWithContext(err).WithExitCode(exitErr.ExitCode())
		}
		return err
	}

	return nil
}

// ensureAccess detects the caller's public IP (or uses the overridden list),
// checks whether it is already permitted, and if not prompts the user before
// calling connector.Update.
func ensureAccess[T resource.Managed](
	ctx context.Context,
	client *api.Client,
	connector accessManager[T],
	res T,
	cmd serviceCmd,
) error {
	var toAdd []meta.IPv4CIDR

	if cmd.AllowedCidrs != nil {
		toAdd = *cmd.AllowedCidrs

		if cidrsPresent(connector.AllowedCIDRs(res), toAdd) {
			cmd.Infof("✅", "specified CIDRs are already allowed")
			return nil
		}
	} else {
		ip, err := ipcheck.New(ipcheck.WithUserAgent(cli.Name)).PublicIP(ctx)
		if err != nil {
			return cli.ErrorWithContext(fmt.Errorf("detecting public IP address: %w", err)).
				WithSuggestions("Are you connected to the internet?")
		}
		if ip.Blocked {
			return cli.ErrorWithContext(fmt.Errorf("public IP seems to be blocked")).
				WithContext("IP", ip.RemoteAddr.String()).
				WithSuggestions("Reach out to support@nine.ch.")
		}
		cmd.Infof("🌐", "detected public IP: %s", ip.RemoteAddr)

		if cidr := ipCoveredByCIDRs(ip.RemoteAddr, connector.AllowedCIDRs(res)); cidr != nil {
			cmd.Infof("✅", "IP %s is already allowedby %s", ip.RemoteAddr, cidr.String())
			return nil
		}

		toAdd = []meta.IPv4CIDR{meta.IPv4CIDR(netip.PrefixFrom(ip.RemoteAddr, 32).String())}
	}

	msg := fmt.Sprintf("Add %v to the allowed CIDRs of %q?", toAdd, res.GetName())
	ok, err := cmd.confirm(msg)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("CIDR addition canceled")
	}

	// Merge with existing CIDRs.
	merged := appendMissing(connector.AllowedCIDRs(res), toAdd)
	if err := connector.Update(ctx, client, res, merged); err != nil {
		return fmt.Errorf("updating allowed CIDRs: %w", err)
	}

	cmd.Infof("ℹ️", "to remove this CIDR later: nctl update %s %s", res.GetObjectKind().GroupVersionKind().Kind, res.GetName())

	return nil
}

// waitForConnectivity dials endpoint in a retry loop until it succeeds or timeout expires.
func waitForConnectivity(ctx context.Context, writer format.Writer, endpoint string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	spinner, err := writer.Spinner(
		format.Progressf("⏳", "waiting for connectivity to %s", endpoint),
		format.Progressf("✅", "connected to %s", endpoint),
	)
	if err != nil {
		return err
	}

	_ = spinner.Start()
	defer func() { _ = spinner.Stop() }()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		attemptCtx, attemptCancel := context.WithTimeout(ctx, 3*time.Second)
		dialErr := dialTCP(attemptCtx, endpoint)
		attemptCancel()
		if dialErr == nil {
			_ = spinner.Stop()
			return nil
		}

		select {
		case <-ctx.Done():
			switch ctx.Err() {
			case context.DeadlineExceeded:
				msg := "timeout waiting for connectivity to %s"
				spinner.StopFailMessage(format.Progressf("", msg, endpoint))
				_ = spinner.StopFail()
				return fmt.Errorf(msg, endpoint)
			default:
				_ = spinner.StopFail()
				return nil
			}
		case <-ticker.C:
		}
	}
}

// checkPath verifies that the named CLI binary is installed and on PATH.
func (cmd serviceCmd) checkPath(name string) error {
	if _, err := cmd.getLookPath()(name); err != nil {
		return cli.ErrorWithContext(fmt.Errorf("%q CLI not found", name)).
			WithSuggestions(
				fmt.Sprintf("Install %q and ensure it is available in your PATH.", name),
			)
	}
	return nil
}

// confirm prints a confirmation prompt. When stdin is not a TTY it opens /dev/tty
// so that piped input (e.g. SQL dumps) does not consume the prompt, mirroring
// the pattern used by git and ssh.
func (cmd serviceCmd) confirm(msg string) (bool, error) {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		tty, err := cmd.openTTY()()
		if err == nil {
			defer tty.Close()
			return cmd.Confirm(format.NewReader(tty), msg)
		}
	}
	return cmd.Confirm(cmd.Reader, msg)
}

// getCredentials fetches the connection secret for the given resource and
// returns the first username/password pair found.
func getCredentials(ctx context.Context, client *api.Client, mg resource.Managed) (string, string, error) {
	secret, err := get.ConnectionSecretMap(ctx, client, mg)
	if err != nil {
		return "", "", fmt.Errorf("getting connection secret: %w", err)
	}

	for user, pw := range secret {
		return user, string(pw), nil
	}

	return "", "", fmt.Errorf("connection secret %q contains no credentials", mg.GetWriteConnectionSecretToReference().Name)
}

// ipCoveredByCIDRs reports whether ip is contained in any of the given CIDRs.
func ipCoveredByCIDRs(ip netip.Addr, cidrs []meta.IPv4CIDR) *netip.Prefix {
	for _, cidr := range cidrs {
		p, err := netip.ParsePrefix(string(cidr))
		if err != nil {
			continue
		}
		if p.Contains(ip) {
			return &p
		}
	}

	return nil
}

// cidrsPresent reports whether all of want are present in current.
func cidrsPresent(current []meta.IPv4CIDR, want []meta.IPv4CIDR) bool {
	set := make(map[meta.IPv4CIDR]struct{}, len(current))
	for _, c := range current {
		set[c] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			return false
		}
	}
	return true
}

// appendMissing appends any CIDRs from add that are not already in current.
func appendMissing(current []meta.IPv4CIDR, add []meta.IPv4CIDR) []meta.IPv4CIDR {
	set := make(map[meta.IPv4CIDR]struct{}, len(current))
	for _, c := range current {
		set[c] = struct{}{}
	}
	result := append([]meta.IPv4CIDR(nil), current...)
	for _, a := range add {
		if _, ok := set[a]; !ok {
			result = append(result, a)
		}
	}
	return result
}

// dialTCP opens a single TCP connection to endpoint, respecting ctx for
// cancellation and deadline.
func dialTCP(ctx context.Context, endpoint string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// quickDial attempts a single TCP connection with a short timeout.
// Returns true when the endpoint is immediately reachable.
func quickDial(ctx context.Context, endpoint string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return dialTCP(ctx, endpoint) == nil
}

// writeCACert decodes a base64-encoded PEM CA certificate and writes it to a
// temporary file, returning the file path along with a cleanup function.
func writeCACert(caCert string) (path string, cleanup func(), err error) {
	if caCert == "" {
		return "", func() {}, nil
	}

	f, err := os.CreateTemp("", "nctl-ca-*.pem")
	if err != nil {
		return "", func() {}, fmt.Errorf("creating CA cert temp file: %w", err)
	}

	if err := get.WriteBase64(f, caCert); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("writing CA cert: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("closing CA cert temp file: %w", err)
	}

	path = f.Name()
	return path, func() { _ = os.Remove(path) }, nil
}
