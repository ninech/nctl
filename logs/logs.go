package logs

import (
	"context"
	"fmt"
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

type logsCmd struct {
	Follow bool          `help:"Follow the logs by live tailing." short:"f"`
	Lines  int           `help:"Amount of lines to output" default:"10" short:"l"`
	Since  time.Duration `help:"Duration how long to look back for logs" short:"s" default:"10m"`
	Output string        `help:"Configures the log output format. ${enum}" short:"o" enum:"default,json" default:"default"`
	out    output.LogOutput
}

func (cmd *logsCmd) Run(ctx context.Context, client *api.Client, labelKey, labelValue string) error {
	query := log.Query{
		// we just query for labelKey=<labelValue>, namespace=<client-ns>. It's
		// technically already scoped to a single namespace as the client is
		// setting the org-id.
		QueryString: fmt.Sprintf(`{%s="%s", namespace="%s"}`, labelKey, labelValue, client.Namespace),
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