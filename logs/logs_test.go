package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/logcli/output"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
)

func TestApplication(t *testing.T) {
	expectedTime := time.Now()
	lines := []string{}

	for i := 0; i < 100; i++ {
		lines = append(lines, fmt.Sprintf("log line %d", i))
	}

	apiClient := &api.Client{
		Project: "default",
		Log:     &log.Client{Client: log.NewFake(t, expectedTime, lines...)},
	}
	ctx := context.Background()

	cases := map[string]struct {
		cmd           logsCmd
		expectedLines int
	}{
		"line limit": {
			cmd: logsCmd{
				Output: "default",
				Lines:  10,
			},
			expectedLines: 10,
		},
		"json output": {
			cmd: logsCmd{
				Output: "json",
				Lines:  8,
			},
			expectedLines: 8,
		},
		"follow": {
			cmd: logsCmd{
				Output: "default",
				Follow: true,
			},
			// note: the fake follow client does not support limiting log
			// lines so we just expect them all to be returned
			expectedLines: len(lines),
		},
		"follow json": {
			cmd: logsCmd{
				Output: "json",
				Follow: true,
			},
			expectedLines: len(lines),
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			out, err := output.NewLogOutput(&buf, log.Mode(tc.cmd.Output), &output.LogOutputOptions{
				NoLabels: true, ColoredOutput: false, Timezone: time.Local,
			})
			if err != nil {
				t.Fatal(err)
			}

			tc.cmd.out = out

			if err := tc.cmd.Run(ctx, apiClient, ApplicationQuery("app-name", "app-ns")); err != nil {
				t.Fatal(err)
			}

			if tc.expectedLines != 0 {
				assert.Equal(t, test.CountLines(buf.String()), tc.expectedLines)
			}

			if tc.cmd.Output == "json" {
				logLine := struct {
					Timestamp time.Time `json:"timestamp"`
					Line      string    `json:"line"`
				}{}
				// iterate lines, unmarshal each and compare them against the input
				outLines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
				for i, line := range outLines {
					assert.NoError(t, json.Unmarshal([]byte(line), &logLine))
					assert.Equal(t, expectedTime.Local(), logLine.Timestamp.Local())
					assert.Equal(t, lines[i], logLine.Line)
				}
			}
			buf.Reset()
		})
	}
}
