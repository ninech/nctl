package get

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type applicationsCmd struct {
	resourceCmd
	BasicAuthCredentials bool `help:"Show the basic auth credentials of the application."`
	DNS                  bool `help:"Show the DNS details for custom hosts."`
}

func (cmd *applicationsCmd) Run(ctx context.Context, c *api.Client, get *Cmd) error {
	return get.listPrint(ctx, c, cmd, api.MatchName(cmd.Name))
}

func (cmd *applicationsCmd) list() client.ObjectList {
	return &apps.ApplicationList{}
}

func (cmd *applicationsCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	appList := list.(*apps.ApplicationList)
	if len(appList.Items) == 0 {
		return out.printEmptyMessage(apps.ApplicationKind, client.Project)
	}

	if cmd.BasicAuthCredentials {
		creds, err := gatherCredentials(ctx, appList.Items, client)
		if len(creds) == 0 {
			out.Printf("no application with basic auth enabled found\n")
			return err
		}
		if printErr := printCredentials(creds, out); printErr != nil {
			err = multierror.Append(err, printErr)
		}
		return err
	}

	if cmd.DNS {
		return printDNSDetails(util.GatherDNSDetails(appList.Items), out)
	}

	switch out.Format {
	case full:
		return printApplication(appList.Items, out, true)
	case noHeader:
		return printApplication(appList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(appList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			appList.GetItems(),
			format.PrintOpts{
				Out:    &out.Writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			},
		)
	case stats:
		return cmd.printStats(ctx, client, appList.Items, out)
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

func printApplication(apps []apps.Application, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "REPLICAS", "WORKERJOBS", "SCHEDULEDJOBS", "HOSTS", "UNVERIFIEDHOSTS")
	}

	for _, app := range apps {
		verifiedHosts := append(util.VerifiedAppHosts(&app), app.Status.AtProvider.CNAMETarget)
		unverifiedHosts := util.UnverifiedAppHosts(&app)
		replicas := 0
		if app.Status.AtProvider.Replicas != nil {
			replicas = int(*app.Status.AtProvider.Replicas)
		}
		workerJobs := fmt.Sprintf("%d", len(app.Status.AtProvider.WorkerJobs))
		scheduledJobs := fmt.Sprintf("%d", len(app.Status.AtProvider.ScheduledJobs))

		out.writeTabRow(app.Namespace, app.Name, fmt.Sprintf("%d", replicas), workerJobs, scheduledJobs, join(verifiedHosts), join(unverifiedHosts))
	}

	return out.tabWriter.Flush()
}

func printCredentials(creds []appCredentials, out *output) error {
	if out.Format == yamlOut {
		return format.PrettyPrintObjects(creds, format.PrintOpts{Out: &out.Writer})
	}
	if out.Format == jsonOut {
		return format.PrettyPrintObjects(
			creds,
			format.PrintOpts{Out: &out.Writer, Format: format.OutputFormatTypeJSON},
		)
	}
	return printCredentialsTabRow(creds, out)
}

func printCredentialsTabRow(creds []appCredentials, out *output) error {
	if out.Format == full {
		out.writeHeader("NAME", "USERNAME", "PASSWORD")
	}

	for _, cred := range creds {
		out.writeTabRow(cred.Project, cred.Application, cred.Username, cred.Password)
	}

	return out.tabWriter.Flush()
}

type appCredentials struct {
	Application    string `json:"application"`
	Project        string `json:"project"`
	util.BasicAuth `json:"basicauth"`
}

func gatherCredentials(ctx context.Context, items []apps.Application, c *api.Client) ([]appCredentials, error) {
	var resultErrors error
	creds := []appCredentials{}
	for _, app := range items {
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
		return util.NoneText
	}
	return strings.Join(list, ",")
}

func printDNSDetails(items []util.DNSDetail, out *output) error {
	if out.Format == yamlOut {
		return format.PrettyPrintObjects(items, format.PrintOpts{Out: &out.Writer})
	}
	if out.Format == jsonOut {
		return format.PrettyPrintObjects(items, format.PrintOpts{Out: &out.Writer, Format: format.OutputFormatTypeJSON})
	}
	return printDNSDetailsTabRow(items, out)
}

func printDNSDetailsTabRow(items []util.DNSDetail, out *output) error {
	if out.Format == full {
		out.writeHeader("NAME", "TXT RECORD", "DNS TARGET")
	}

	for _, item := range items {
		out.writeTabRow(item.Project, item.Application, item.TXTRecord, item.CNAMETarget)
	}
	if err := out.tabWriter.Flush(); err != nil {
		return err
	}

	out.Printf("\nVisit %s to see instructions on how to setup custom hosts\n", util.DNSSetupURL)
	return nil
}

func sizeForWorkerJob(release *apps.Release, workerJobName string) *apps.ApplicationSize {
	for _, wj := range release.Spec.ForProvider.Configuration.WithoutOrigin().WorkerJobs {
		if wj.Name == workerJobName {
			return wj.Size
		}
	}
	return nil
}

func sizeForScheduledJob(release *apps.Release, scheduledJobName string) *apps.ApplicationSize {
	for _, sj := range release.Spec.ForProvider.Configuration.WithoutOrigin().ScheduledJobs {
		if sj.Name == scheduledJobName {
			return sj.Size
		}
	}
	return nil
}

func (cmd *applicationsCmd) printStats(ctx context.Context, c *api.Client, appList []apps.Application, out *output) error {
	scheme := runtime.NewScheme()
	if err := metricsv1beta1.AddToScheme(scheme); err != nil {
		return err
	}

	runtimeClient, err := c.DeploioRuntimeClient(ctx, scheme)
	if err != nil {
		return err
	}
	out.writeHeader("NAME", "REPLICA", "STATUS", "CPU", "CPU%", "MEMORY", "MEMORY%", "RESTARTS", "LASTEXITCODE")

	type statsObservation struct {
		name string
		size apps.ApplicationSize
		apps.ReplicaObservation
	}

	for _, app := range appList {
		rel, err := util.ApplicationLatestRelease(ctx, c, api.ObjectName(&app))
		if err != nil {
			out.Warningf("unable to get latest release for app %s\n", c.Name(app.Name))
			continue
		}

		var observations []statsObservation
		appSize := rel.Spec.ForProvider.Configuration.WithoutOrigin().Size

		// we first gather the normal application replicas
		for _, appReplica := range rel.Status.AtProvider.ReplicaObservation {
			observations = append(observations, statsObservation{
				name:               app.Name,
				size:               appSize,
				ReplicaObservation: appReplica,
			})
		}
		// we now handle the worker jobs
		for _, wjs := range rel.Status.AtProvider.WorkerJobStatus {
			wjSize := sizeForWorkerJob(rel, wjs.Name)
			if wjSize == nil {
				wjSize = ptr.To(appSize)
			}
			for _, replicaObs := range wjs.ReplicaObservation {
				observations = append(observations, statsObservation{
					name:               wjs.Name,
					ReplicaObservation: replicaObs,
					size:               *wjSize,
				})
			}
		}
		// ..and finally the scheduled jobs
		for _, sjs := range rel.Status.AtProvider.ScheduledJobStatus {
			sjSize := sizeForScheduledJob(rel, sjs.Name)
			if sjSize == nil {
				sjSize = ptr.To(appSize)
			}
			for _, replicaObs := range sjs.ReplicaObservation {
				observations = append(observations, statsObservation{
					name:               sjs.Name,
					ReplicaObservation: replicaObs,
					size:               *sjSize,
				})
			}
		}

		for _, statsObservation := range observations {
			podMetrics := metricsv1beta1.PodMetrics{}
			if err := runtimeClient.Get(
				ctx,
				api.NamespacedName(statsObservation.ReplicaName, app.Namespace),
				&podMetrics,
			); err != nil {
				out.Warningf("unable to get metrics for replica %s\n", statsObservation.ReplicaName)
			}

			maxResources := apps.AppResources[statsObservation.size]
			// We expect exactly one container, fall back to [util.NoneText] if that's
			// not the case. The container might simply not have any metrics yet.
			cpuUsage, cpuPercentage := util.NoneText, util.NoneText
			memoryUsage, memoryPercentage := util.NoneText, util.NoneText
			if len(podMetrics.Containers) == 1 {
				cpu := podMetrics.Containers[0].Usage[corev1.ResourceCPU]
				cpuUsage = formatQuantity(corev1.ResourceCPU, cpu)
				cpuPercentage = formatPercentage(cpu.MilliValue(), maxResources.Cpu().MilliValue())
				memory := podMetrics.Containers[0].Usage[corev1.ResourceMemory]
				memoryUsage = formatQuantity(corev1.ResourceMemory, memory)
				memoryPercentage = formatPercentage(memory.MilliValue(), maxResources.Memory().MilliValue())
			}

			out.writeTabRow(
				app.Namespace, statsObservation.name,
				statsObservation.ReplicaName,
				string(statsObservation.Status),
				cpuUsage,
				cpuPercentage,
				memoryUsage,
				memoryPercentage,
				formatRestartCount(statsObservation.ReplicaObservation),
				formatExitCode(statsObservation.ReplicaObservation),
			)
		}
	}
	return out.tabWriter.Flush()
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
