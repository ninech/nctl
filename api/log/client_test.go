package log

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/loki/v3/pkg/logcli/output"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	t.Parallel()

	is := require.New(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	expectedTime := time.Now()
	expectedLine := "hello loki client"

	c := &Client{Client: NewFake(t, expectedTime, expectedLine)}

	var buf bytes.Buffer
	out, err := output.NewLogOutput(&buf, "default", &output.LogOutputOptions{
		NoLabels: true, ColoredOutput: false, Timezone: time.Local,
	})
	is.NoError(err)

	is.NoError(c.QueryRange(ctx, out, Query{Limit: 10}))

	is.Equal(fmt.Sprintf("%s %s\n", expectedTime.Local().Format(time.RFC3339), expectedLine), buf.String())
	buf.Reset()

	is.NoError(c.TailQuery(ctx, 0, out, Query{QueryString: "{app=\"test\"}", Limit: 10}))
	is.Equal(fmt.Sprintf("%s %s\n", expectedTime.Local().Format(time.RFC3339), expectedLine), buf.String())
}
