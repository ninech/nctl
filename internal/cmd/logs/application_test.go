package logs

import (
	"bytes"
	"testing"
	"time"

	"github.com/grafana/loki/v3/pkg/loghttp"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api/log"
	"github.com/stretchr/testify/require"
)

func TestAppLogTypeMatchers(t *testing.T) {
	t.Parallel()

	cases := map[appLogType][]log.Matcher{
		logTypeAll: nil,
		logTypeApp: {
			`deploy_job=""`, `worker_job=""`, `scheduled_job=""`, `build=""`,
		},
		logTypeBuild:        {`build!=""`},
		logTypeDeployJob:    {`deploy_job!=""`},
		logTypeWorkerJob:    {`worker_job!=""`},
		logTypeScheduledJob: {`scheduled_job!=""`},
	}
	for logType, expected := range cases {
		t.Run(string(logType), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, expected, logType.matchers())
		})
	}
}

func TestApplicationLogLabels(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		labels   loghttp.LabelSet
		expected string
	}{
		"replica": {
			labels:   loghttp.LabelSet{apps.LogLabelReplica: "web-1"},
			expected: `replica="web-1"`,
		},
		"build": {
			labels:   loghttp.LabelSet{apps.LogLabelBuild: "build-1"},
			expected: `build="build-1"`,
		},
		"worker job": {
			labels:   loghttp.LabelSet{apps.LogLabelWorkerJob: "worker"},
			expected: `worker_job="worker"`,
		},
		"deploy job": {
			labels:   loghttp.LabelSet{apps.LogLabelDeployJob: "migrate"},
			expected: `deploy_job="migrate"`,
		},
		"scheduled job": {
			labels:   loghttp.LabelSet{apps.LogLabelScheduledJob: "cleanup"},
			expected: `scheduled_job="cleanup"`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			out, err := log.NewOutput(&buf, log.Mode("default"), false, applicationLogLabels...)
			require.NoError(t, err)

			tc.labels[apps.LogLabelProject] = "project"
			out.FormatAndPrintln(time.Now(), tc.labels, 0, "line")

			require.Contains(t, buf.String(), tc.expected)
			require.NotContains(t, buf.String(), apps.LogLabelProject)
		})
	}
}
