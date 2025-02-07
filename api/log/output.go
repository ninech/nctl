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
	out       output.LogOutput
	labels    map[string]struct{}
	lineCount int
}

func (o *filteredOutput) FormatAndPrintln(ts time.Time, lbls loghttp.LabelSet, maxLabelsLen int, line string) {
	for k := range lbls {
		if _, ok := o.labels[k]; !ok {
			delete(lbls, k)
		}
	}
	o.out.FormatAndPrintln(ts, lbls, maxLabelsLen, line)
	o.lineCount++
}

func (o filteredOutput) WithWriter(w io.Writer) output.LogOutput {
	return o.out.WithWriter(w)
}

func (o filteredOutput) LineCount() int {
	return o.lineCount
}

type Output interface {
	output.LogOutput
	// LineCount returns the amount of lines the output has processed
	LineCount() int
}

func NewStdOut(mode string, noLabels bool, labels ...string) (Output, error) {
	return NewOutput(os.Stdout, mode, noLabels, labels...)
}

func NewOutput(w io.Writer, mode string, noLabels bool, labels ...string) (Output, error) {
	out, err := output.NewLogOutput(w, mode, &output.LogOutputOptions{
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
