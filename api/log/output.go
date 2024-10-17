package log

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/grafana/loki/pkg/logcli/output"
	"github.com/grafana/loki/pkg/loghttp"
)

type filteredOutput struct {
	out    output.LogOutput
	labels map[string]struct{}
}

func (o *filteredOutput) FormatAndPrintln(ts time.Time, lbls loghttp.LabelSet, maxLabelsLen int, line string) {
	for k := range lbls {
		if _, ok := o.labels[k]; !ok {
			delete(lbls, k)
		}
	}
	o.out.FormatAndPrintln(ts, lbls, maxLabelsLen, line)
}

func (o filteredOutput) WithWriter(w io.Writer) output.LogOutput {
	return o.out.WithWriter(w)
}

func NewStdOut(mode string, noLabels bool, labels ...string) (output.LogOutput, error) {
	out, err := output.NewLogOutput(os.Stdout, mode, &output.LogOutputOptions{
		NoLabels: noLabels, ColoredOutput: true, Timezone: time.Local,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create log output: %s", err)
	}

	keys := map[string]struct{}{}
	for _, label := range labels {
		keys[label] = struct{}{}
	}
	return &filteredOutput{out: out, labels: keys}, nil
}
