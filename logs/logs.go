package logs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
)

type Cmd struct {
	Applications applicationCmd `cmd:"" group:"deplo.io" name:"application" aliases:"app,application" help:"Get deplo.io Application logs."`
	Builds       buildCmd       `cmd:"" group:"deplo.io" name:"build" help:"Get deplo.io Build logs."`
}

type resourceCmd struct {
	Name string `arg:"" predictor:"resource_name" help:"Name of the resource." default:""`
}

type logsCmd struct {
	Follow   bool          `help:"Follow the logs by live tailing." short:"f"`
	Lines    int           `help:"Amount of lines to output" default:"50" short:"l"`
	Since    time.Duration `help:"Duration how long to look back for logs" short:"s" default:"${log_retention}"`
	From     time.Time     `help:"Ignore since flag and start looking for logs at this absolute time (RFC3339)" placeholder:"2025-01-01T14:00:00+01:00"`
	To       time.Time     `help:"Ignore since flag and stop looking for logs at this absolute time (RFC3339)" placeholder:"2025-01-01T15:00:00+01:00"`
	Output   string        `help:"Configures the log output format. ${enum}" short:"o" enum:"default,json" default:"default"`
	NoLabels bool          `help:"disable labels in log output"`
	out      log.Output
}

// 30 days, we hardcode this for now as it's not possible to customize this on
// deplo.io. We'll need to revisit this if we ever make this configurable.
var logRetention = time.Duration(time.Hour * 24 * 30)

func (cmd *logsCmd) Run(ctx context.Context, client *api.Client, queryString string, labels ...string) error {
	now := time.Now()
	start, end := now.Add(-cmd.Since), now
	if !cmd.From.IsZero() {
		start = cmd.From
	}
	if !cmd.To.IsZero() {
		end = cmd.To
	}
	if now.Sub(start) > logRetention {
		return fmt.Errorf("the logs requested exceed the retention period of %.f days", logRetention.Hours()/24)
	}

	query := log.Query{
		QueryString: queryString,
		Limit:       cmd.Lines,
		Start:       start,
		End:         end,
		Direction:   logproto.BACKWARD,
		Quiet:       true,
	}

	out, err := log.NewStdOut(log.Mode(cmd.Output), cmd.NoLabels, labels...)
	if err != nil {
		return err
	}

	if cmd.out != nil {
		out = cmd.out
	}

	if cmd.Follow {
		return client.Log.TailQuery(ctx, 0, out, query)
	}

	if err := client.Log.QueryRange(ctx, out, query); err != nil {
		return err
	}
	if out.LineCount() == 0 {
		return fmt.Errorf("no logs found between %s and %s", start.Format(time.RFC3339), end.Format(time.RFC3339))
	}

	return nil
}

type queryOperator string

const (
	opEquals    queryOperator = "="
	opNotEquals queryOperator = "!="
)

func queryExpr(operator queryOperator, key, value string) string {
	return fmt.Sprintf(`%s%s"%s"`, key, operator, value)
}

func buildQuery(expr ...string) string {
	return "{" + strings.Join(expr, ",") + "}"
}

func inProject(project string) string {
	return queryExpr(opEquals, "namespace", project)
}

// KongVars returns all variables which are used in the logs commands
func KongVars() kong.Vars {
	result := make(kong.Vars)
	result["log_retention"] = logRetention.String()
	return result
}
