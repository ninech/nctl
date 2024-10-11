package get

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/go-multierror"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

type applicationsCmd struct {
	resourceCmd
	BasicAuthCredentials bool `help:"Show the basic auth credentials of the application."`
	DNS                  bool `help:"Show the DNS details for custom hosts."`
	out                  io.Writer
}

func (cmd *applicationsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	appList := &apps.ApplicationList{}
	if err := get.list(ctx, client, appList, matchName(cmd.Name)); err != nil {
		return err
	}

	if len(appList.Items) == 0 {
		get.printEmptyMessage(cmd.out, apps.ApplicationKind, client.Project)
		return nil
	}

	if cmd.BasicAuthCredentials {
		creds, err := gatherCredentials(ctx, appList.Items, client)
		if len(creds) == 0 {
			fmt.Fprintf(defaultOut(cmd.out), "no application with basic auth enabled found\n")
			return err
		}
		if printErr := printCredentials(creds, get, defaultOut(cmd.out)); printErr != nil {
			err = multierror.Append(err, printErr)
		}
		return err
	}

	if cmd.DNS {
		return printDNSDetails(util.GatherDNSDetails(appList.Items), get, defaultOut(cmd.out))
	}

	switch get.Output {
	case full:
		return printApplication(appList.Items, get, defaultOut(cmd.out), true)
	case noHeader:
		return printApplication(appList.Items, get, defaultOut(cmd.out), false)
	case yamlOut:
		return format.PrettyPrintObjects(appList.GetItems(), format.PrintOpts{Out: defaultOut(cmd.out)})
	case stats:
		return cmd.printStats(ctx, client, appList.Items, get, defaultOut(cmd.out))
	}

	return nil
}

func (cmd *applicationsCmd) Help() string {
	return "To get an overview of the app and replica usage, use the flag '-o stats':\n" +
		"\tREPLICA: The name of the app replica.\n" +
		"\tSTATUS: Current status of the replica.\n" +
		"\tCPU: Current CPU usage in millicores (1000m is a full CPU core).\n" +
		"\tCPU%: Current CPU usage relative to the app size. This can be over 100% as Deploio allows bursting.\n" +
		"\tMEMORY: Current Memory usage in MiB.\n" +
		"\tMEMORY%: Current Memory relative to the app size. This can be over 100% as Deploio allows bursting.\n" +
		"\tRESTARTS: The amount of times the replica has been restarted.\n" +
		"\tLASTEXITCODE: The exit code the last time the replica restarted. This can give an indication on why the replica is restarting."
}

func printApplication(apps []apps.Application, get *Cmd, out io.Writer, header bool) error {
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "HOSTS", "UNVERIFIED_HOSTS")
	}

	for _, app := range apps {
		verifiedHosts := append(util.VerifiedAppHosts(&app), app.Status.AtProvider.CNAMETarget)
		unverifiedHosts := util.UnverifiedAppHosts(&app)

		get.writeTabRow(w, app.Namespace, app.Name, join(verifiedHosts), join(unverifiedHosts))
	}

	return w.Flush()
}

func printCredentials(creds []appCredentials, get *Cmd, out io.Writer) error {
	if get.Output == yamlOut {
		return format.PrettyPrintObjects(creds, format.PrintOpts{Out: out})
	}
	return printCredentialsTabRow(creds, get, out)
}

func printCredentialsTabRow(creds []appCredentials, get *Cmd, out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)

	if get.Output == full {
		get.writeHeader(w, "NAME", "USERNAME", "PASSWORD")
	}

	for _, cred := range creds {
		get.writeTabRow(w, cred.Project, cred.Application, cred.Username, cred.Password)
	}

	return w.Flush()
}

type appCredentials struct {
	Application string `yaml:"application"`
	Project     string `yaml:"project"`
	util.BasicAuth
}

func gatherCredentials(ctx context.Context, items []apps.Application, c *api.Client) ([]appCredentials, error) {
	var resultErrors error
	creds := []appCredentials{}
	for _, app := range items {
		app := app
		if app.Status.AtProvider.BasicAuthSecret == nil {
			// the app has no basic auth configured so we skip it
			// in the output
			continue
		}
		basicAuth, err := util.NewBasicAuthFromSecret(
			ctx,
			app.Status.AtProvider.BasicAuthSecret.InNamespace(&app),
			c,
		)
		if err != nil {
			resultErrors = multierror.Append(
				resultErrors,
				fmt.Errorf("can not gather credentials for application %q: %w", app.Name, err),
			)
			continue
		}
		creds = append(creds, appCredentials{Application: app.Name, Project: app.Namespace, BasicAuth: *basicAuth})
	}
	return creds, resultErrors
}

func join(list []string) string {
	if len(list) == 0 {
		return "none"
	}
	return strings.Join(list, ",")
}

func printDNSDetails(items []util.DNSDetail, get *Cmd, out io.Writer) error {
	if get.Output == yamlOut {
		return format.PrettyPrintObjects(items, format.PrintOpts{Out: out})
	}
	return printDNSDetailsTabRow(items, get, out)
}

func printDNSDetailsTabRow(items []util.DNSDetail, get *Cmd, out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)

	if get.Output == full {
		get.writeHeader(w, "NAME", "TXT RECORD", "DNS TARGET")
	}

	for _, item := range items {
		get.writeTabRow(w, item.Project, item.Application, item.TXTRecord, item.CNAMETarget)
	}

	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\nVisit %s to see instructions on how to setup custom hosts\n", util.DNSSetupURL)

	return nil
}

func (cmd *applicationsCmd) printStats(ctx context.Context, c *api.Client, appList []apps.Application, get *Cmd, out io.Writer) error {
	scheme := runtime.NewScheme()
	if err := metricsv1beta1.AddToScheme(scheme); err != nil {
		return err
	}

	runtimeClient, err := c.DeploioRuntimeClient(ctx, scheme)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	get.writeHeader(w, "NAME", "REPLICA", "STATUS", "CPU", "CPU%", "MEMORY", "MEMORY%", "RESTARTS", "LASTEXITCODE")

	for _, app := range appList {
		replicas, err := util.ApplicationReplicas(ctx, c, api.ObjectName(&app))
		if err != nil {
			format.PrintWarningf("unable to get replicas for app %s\n", c.Name(app.Name))
			continue
		}

		if len(replicas) == 0 {
			continue
		}

		for _, replica := range replicas {
			podMetrics := metricsv1beta1.PodMetrics{}
			if err := runtimeClient.Get(ctx, api.NamespacedName(replica.ReplicaName, app.Namespace), &podMetrics); err != nil {
				format.PrintWarningf("unable to get metrics for replica %s\n", replica.ReplicaName)
			}

			appResources := apps.AppResources[app.Status.AtProvider.Size]
			// We expect exactly one container, fall back to [util.NoneText] if that's
			// not the case. The container might simply not have any metrics yet.
			cpuUsage, cpuPercentage := util.NoneText, util.NoneText
			memoryUsage, memoryPercentage := util.NoneText, util.NoneText
			if len(podMetrics.Containers) == 1 {
				cpu := podMetrics.Containers[0].Usage[corev1.ResourceCPU]
				cpuUsage = formatQuantity(corev1.ResourceCPU, cpu)
				cpuPercentage = formatPercentage(cpu.MilliValue(), appResources.Cpu().MilliValue())
				memory := podMetrics.Containers[0].Usage[corev1.ResourceMemory]
				memoryUsage = formatQuantity(corev1.ResourceMemory, memory)
				memoryPercentage = formatPercentage(memory.MilliValue(), appResources.Memory().MilliValue())
			}

			get.writeTabRow(
				w, c.Project, app.Name,
				replica.ReplicaName,
				string(replica.Status),
				cpuUsage,
				cpuPercentage,
				memoryUsage,
				memoryPercentage,
				formatRestartCount(replica),
				formatExitCode(replica),
			)
		}
	}
	return w.Flush()
}

// formatQuantity formats cpu/memory into human readable form. Adapted from
// https://github.com/kubernetes/kubectl/blob/v0.31.1/pkg/metricsutil/metrics_printer.go#L209
func formatQuantity(resourceType corev1.ResourceName, quantity resource.Quantity) string {
	switch resourceType {
	case corev1.ResourceCPU:
		return fmt.Sprintf("%vm", quantity.MilliValue())
	case corev1.ResourceMemory:
		return fmt.Sprintf("%vMiB", quantity.Value()/toMiB(1))
	default:
		return fmt.Sprintf("%v", quantity.Value())
	}
}

func formatPercentage(val, total int64) string {
	if total == 0 {
		return util.NoneText
	}
	return fmt.Sprintf("%.1f", float64(val)/float64(total)*100) + "%"
}

func toMiB(val int64) int64 {
	return val * 1024 * 1024
}

func formatExitCode(replica apps.ReplicaObservation) string {
	lastExitCode := util.NoneText

	if replica.LastExitCode != nil {
		lastExitCode = strconv.Itoa(int(*replica.LastExitCode))
		// not exactly guaranteed but 137 is usually caused by the OOM killer
		if *replica.LastExitCode == 137 {
			lastExitCode = lastExitCode + " (Out of memory)"
		}
	}
	return lastExitCode
}

func formatRestartCount(replica apps.ReplicaObservation) string {
	restartCount := util.NoneText
	if replica.RestartCount != nil {
		restartCount = strconv.Itoa(int(*replica.RestartCount))
	}
	return restartCount
}
