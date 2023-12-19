package log

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grafana/dskit/backoff"
	logclient "github.com/grafana/loki/pkg/logcli/client"
	"github.com/grafana/loki/pkg/logcli/output"
	"github.com/grafana/loki/pkg/loghttp"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/grafana/loki/pkg/logqlmodel"
	"github.com/grafana/loki/pkg/util/unmarshal"
	"github.com/prometheus/common/config"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

type Client struct {
	logclient.Client
	StdOut output.LogOutput
}

type Query struct {
	QueryString string
	Start       time.Time
	End         time.Time
	Limit       int
	Step        time.Duration
	Interval    time.Duration
	Quiet       bool
	Direction   logproto.Direction
}

// NewClient returns a new log API client.
func NewClient(address, token, orgID string, insecure bool) (*Client, error) {
	out, err := StdOut("default")
	if err != nil {
		return nil, err
	}

	tls := config.TLSConfig{}
	if insecure {
		tls.InsecureSkipVerify = true
	}

	return &Client{
		StdOut: out,
		Client: &logclient.DefaultClient{
			Address:     address,
			BearerToken: token,
			OrgID:       orgID,
			TLSConfig:   tls,
		},
	}, nil
}

// Mode translates the mode to lokis terminology.
func Mode(m string) string {
	// the json output mode is called "jsonl" for some reason
	if m == "json" {
		return "jsonl"
	}

	return m
}

// StdOut sets up an stdout log output with the specified mode.
func StdOut(mode string) (output.LogOutput, error) {
	out, err := output.NewLogOutput(os.Stdout, mode, &output.LogOutputOptions{
		NoLabels: true, ColoredOutput: true, Timezone: time.Local,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create log output: %s", err)
	}

	return out, nil
}

// QueryRange queries logs within a specific time range.
func (c *Client) QueryRange(ctx context.Context, out output.LogOutput, q Query) error {
	resp, err := c.Client.QueryRange(q.QueryString, q.Limit, q.Start, q.End, q.Direction, q.Step, q.Interval, q.Quiet)
	if err != nil {
		return err
	}

	return printResult(resp.Data.Result, out)
}

// QueryRangeWithRetry queries logs within a specific time range with a retry
// in case of an error or not finding any logs.
func (c *Client) QueryRangeWithRetry(ctx context.Context, out output.LogOutput, q Query) error {
	return retry.OnError(
		wait.Backoff{
			Steps:    5,
			Duration: 200 * time.Millisecond,
			Factor:   2.0,
			Jitter:   0.1,
			Cap:      10 * time.Second,
		},
		func(err error) bool {
			// retry regardless of the error
			return true
		},
		func() error {
			resp, err := c.Client.QueryRange(q.QueryString, q.Limit, q.Start, q.End, q.Direction, q.Step, q.Interval, q.Quiet)
			if err != nil {
				return err
			}

			switch streams := resp.Data.Result.(type) {
			case loghttp.Streams:
				if len(streams) == 0 {
					return fmt.Errorf("received no log streams")
				}
			}

			return printResult(resp.Data.Result, out)
		})
}

func printResult(value loghttp.ResultValue, out output.LogOutput) error {
	switch value.Type() {
	case logqlmodel.ValueTypeStreams:
		printStream(value.(loghttp.Streams), out)
	default:
		return fmt.Errorf("unable to print unsupported type: %v", value.Type())
	}
	return nil
}

func printStream(streams loghttp.Streams, out output.LogOutput) {
	allEntries := []loghttp.Entry{}
	for _, s := range streams {
		allEntries = append(allEntries, s.Entries...)
	}

	sort.Slice(allEntries, func(i, j int) bool { return allEntries[i].Timestamp.Before(allEntries[j].Timestamp) })

	for _, e := range allEntries {
		out.FormatAndPrintln(e.Timestamp, nil, 0, e.Line)
	}
}

// TailQuery tails logs using the loki websocket endpoint.
// This has been adapted from https://github.com/grafana/loki/blob/v2.8.2/pkg/logcli/query/tail.go#L22
// as it directly prints out messages using builtin log, which we don't want.
func (c *Client) TailQuery(ctx context.Context, delayFor time.Duration, out output.LogOutput, q Query) error {
	conn, err := c.LiveTailQueryConn(q.QueryString, delayFor, q.Limit, q.Start, q.Quiet)
	if err != nil {
		return fmt.Errorf("tailing logs failed: %w", err)
	}

	go func() {
		<-ctx.Done()
		// if sending the close message fails there's not much we can do.
		// Printing the message would probably confuse the user more than
		// anything.
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}()

	tailResponse := new(loghttp.TailResponse)
	lastReceivedTimestamp := q.Start

	for {
		err := unmarshal.ReadTailResponseJSON(tailResponse, conn)
		if err != nil {
			// Check if the websocket connection closed unexpectedly. If so, retry.
			// The connection might close unexpectedly if the querier handling the tail request
			// in Loki stops running. The following error would be printed:
			// "websocket: close 1006 (abnormal closure): unexpected EOF"
			if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
				// Close previous connection. If it fails to close the connection it should be fine as it is already broken.
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

				// Try to re-establish the connection up to 5 times.
				backoff := backoff.New(context.Background(), backoff.Config{
					MinBackoff: 1 * time.Second,
					MaxBackoff: 10 * time.Second,
					MaxRetries: 5,
				})

				for backoff.Ongoing() {
					conn, err = c.LiveTailQueryConn(q.QueryString, delayFor, q.Limit, lastReceivedTimestamp, q.Quiet)
					if err == nil {
						break
					}

					backoff.Wait()
				}

				if err = backoff.Err(); err != nil {
					return fmt.Errorf("error recreating tailing connection: %w", err)
				}

				continue
			}

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return nil
			}

			return fmt.Errorf("error reading stream: %w", err)
		}

		for _, stream := range tailResponse.Streams {
			for _, entry := range stream.Entries {
				out.FormatAndPrintln(entry.Timestamp, stream.Labels, 0, entry.Line)
				lastReceivedTimestamp = entry.Timestamp
			}
		}
	}
}
