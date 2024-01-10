package log

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/logcli/output"
	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	expectedTime := time.Now()
	expectedLine := "hello loki client"

	c := &Client{Client: NewFake(t, expectedTime, expectedLine)}

	var buf bytes.Buffer
	out, err := output.NewLogOutput(&buf, "default", &output.LogOutputOptions{
		NoLabels: true, ColoredOutput: false, Timezone: time.Local,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := c.QueryRange(ctx, out, Query{Limit: 10}); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, fmt.Sprintf("%s %s\n", expectedTime.Local().Format(time.RFC3339), expectedLine), buf.String())
	buf.Reset()

	if err := c.TailQuery(ctx, 0, out, Query{QueryString: "{app=\"test\"}", Limit: 10}); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, fmt.Sprintf("%s %s\n", expectedTime.Local().Format(time.RFC3339), expectedLine), buf.String())
}
