package logs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/grafana/loki/pkg/logcli/output"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
)

type Cmd struct {
	Applications applicationCmd `cmd:"" group:"deplo.io" name:"application" aliases:"app" help:"Get deplo.io Application logs. (Beta - requires access)"`
	Builds       buildCmd       `cmd:"" group:"deplo.io" name:"build" help:"Get deplo.io Build logs. (Beta - requires access)"`
}

type resourceCmd struct {
	Name string `arg:"" help:"Name of the resource." default:""`
}

type logsCmd struct {
	Follow bool          `help:"Follow the logs by live tailing." short:"f"`
	Lines  int           `help:"Amount of lines to output" default:"20" short:"l"`
	Since  time.Duration `help:"Duration how long to look back for logs" short:"s" default:"60m"`
	Output string        `help:"Configures the log output format. ${enum}" short:"o" enum:"default,json" default:"default"`
	out    output.LogOutput
}

const (
	phaseLabel = "phase"
)

func (cmd *logsCmd) Run(ctx context.Context, client *api.Client, queryString string) error {
	query := log.Query{
		QueryString: queryString,
		Limit:       cmd.Lines,
		Start:       time.Now().Add(-cmd.Since),
		End:         time.Now(),
		Direction:   logproto.BACKWARD,
		Quiet:       true,
	}

	out, err := log.StdOut(log.Mode(cmd.Output))
	if err != nil {
		return err
	}

	if cmd.out != nil {
		out = cmd.out
	}

	if cmd.Follow {
		return client.Log.TailQuery(ctx, 0, out, query)
	}

	return client.Log.QueryRange(ctx, out, query)
}

// queryString can take a set of labels (key, value) to query logs for. The
// namespace label will always be included and takes on the value of the
// project.
func queryString(labels map[string]string, project string) string {
	pairs := []string{}

	labels["namespace"] = project
	for k, v := range labels {
		pairs = append(pairs, fmt.Sprintf(`%s="%s"`, k, v))
	}
	sort.Strings(pairs)

	return "{" + strings.Join(pairs, ",") + "}"
}
