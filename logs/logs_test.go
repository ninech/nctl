package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	expectedTime := time.Now()
	lines := []string{}

	for i := range 100 {
		lines = append(lines, fmt.Sprintf("log line %d", i))
	}

	apiClient := &api.Client{
		Project: "default",
		Log:     &log.Client{Client: log.NewFake(t, expectedTime, lines...)},
	}
	ctx := context.Background()

	cases := map[string]struct {
		cmd                 logsCmd
		expectedLines       int
		expectedErrContains string
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
				Lines:  50,
			},
			expectedLines: 50,
		},
		"follow json": {
			cmd: logsCmd{
				Output: "json",
				Follow: true,
				Lines:  len(lines),
			},
			expectedLines: len(lines),
		},
		"exceeds retention": {
			cmd: logsCmd{
				Output: "default",
				Lines:  len(lines),
				Since:  logRetention * 2,
			},
			expectedErrContains: "the logs requested exceed the retention period",
		},
		"from/to flags override since": {
			cmd: logsCmd{
				Output: "default",
				Lines:  len(lines),
				Since:  logRetention * 2,
				From:   time.Now().Add(-time.Hour),
				To:     time.Now(),
			},
			expectedLines: len(lines),
		},
		"from flag alone overrides since": {
			cmd: logsCmd{
				Output: "default",
				Lines:  len(lines),
				Since:  logRetention * 2,
				From:   time.Now().Add(-time.Hour),
			},
			expectedLines: len(lines),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			out, err := log.NewOutput(&buf, log.Mode(tc.cmd.Output), true)
			if err != nil {
				t.Fatal(err)
			}

			tc.cmd.out = out

			if err := tc.cmd.Run(ctx, apiClient, ApplicationQuery("app-name", "app-ns")); err != nil {
				if tc.expectedErrContains != "" {
					assert.ErrorContains(t, err, tc.expectedErrContains)
				} else {
					assert.NoError(t, err)
				}
			}

			if tc.expectedLines != 0 {
				assert.Equal(t, out.LineCount(), tc.expectedLines)
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

func TestMatchLabels(t *testing.T) {
	assert.Equal(t,
		buildQuery(queryExpr(opEquals, apps.LogLabelApplication, "some-app"), inProject("default")),
		`{app="some-app",namespace="default"}`,
	)
}
