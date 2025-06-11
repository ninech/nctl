package log

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grafana/dskit/backoff"
	logclient "github.com/grafana/loki/v3/pkg/logcli/client"
	"github.com/grafana/loki/v3/pkg/logcli/output"
	"github.com/grafana/loki/v3/pkg/loghttp"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/grafana/loki/v3/pkg/logqlmodel"
	"github.com/grafana/loki/v3/pkg/util/unmarshal"
	"github.com/prometheus/common/config"
)

type tokenFunc func(ctx context.Context) string

type Client struct {
	bearerTokenFunc tokenFunc
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
func NewClient(address string, tokenFunc tokenFunc, orgID string, insecure bool) (*Client, error) {
	out, err := StdOut("default")
	if err != nil {
		return nil, err
	}

	tls := config.TLSConfig{}
	if insecure {
		tls.InsecureSkipVerify = true
	}

	return &Client{
		bearerTokenFunc: tokenFunc,
		StdOut:          out,
		Client: &logclient.DefaultClient{
			Address:   address,
			OrgID:     orgID,
			TLSConfig: tls,
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

func (c *Client) refreshToken(ctx context.Context) {
	if c.bearerTokenFunc == nil {
		return
	}
	if defaultClient, is := c.Client.(*logclient.DefaultClient); is {
		defaultClient.BearerToken = c.bearerTokenFunc(ctx)
	}
}

// QueryRange queries logs within a specific time range and prints the result.
func (c *Client) QueryRange(ctx context.Context, out output.LogOutput, q Query) error {
	c.refreshToken(ctx)
	resp, err := c.Client.QueryRange(q.QueryString, q.Limit, q.Start, q.End, q.Direction, q.Step, q.Interval, q.Quiet)
	if err != nil {
		return err
	}

	return printResult(resp.Data.Result, out)
}

// QueryRangeResponse queries logs within a specific time range and returns the response.
func (c *Client) QueryRangeResponse(ctx context.Context, q Query) (*loghttp.QueryResponse, error) {
	c.refreshToken(ctx)
	return c.Client.QueryRange(q.QueryString, q.Limit, q.Start, q.End, q.Direction, q.Step, q.Interval, q.Quiet)
}

// LiveTailQueryConn does a live tailing with a specific query.
func (c *Client) LiveTailQueryConn(ctx context.Context, queryStr string, delayFor time.Duration, limit int, start time.Time, quiet bool) (*websocket.Conn, error) {
	c.refreshToken(ctx)
	return c.Client.LiveTailQueryConn(queryStr, delayFor, limit, start, quiet)
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

type labelEntry struct {
	loghttp.Entry
	labels loghttp.LabelSet
}

func printStream(streams loghttp.Streams, out output.LogOutput) {
	sortedEntries := []labelEntry{}
	for _, s := range streams {
		for _, entry := range s.Entries {
			sortedEntries = append(sortedEntries, labelEntry{Entry: entry, labels: s.Labels})
		}
	}

	sort.Slice(sortedEntries, func(i, j int) bool { return sortedEntries[i].Timestamp.Before(sortedEntries[j].Timestamp) })

	for _, e := range sortedEntries {
		out.FormatAndPrintln(e.Timestamp, e.labels, 0, e.Line)
	}
}

// TailQuery tails logs using the loki websocket endpoint.
// This has been adapted from https://github.com/grafana/loki/blob/v2.8.2/pkg/logcli/query/tail.go#L22
// as it directly prints out messages using builtin log, which we don't want.
func (c *Client) TailQuery(ctx context.Context, delayFor time.Duration, out output.LogOutput, q Query) error {
	conn, err := c.LiveTailQueryConn(ctx, q.QueryString, delayFor, q.Limit, q.Start, q.Quiet)
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

	lastReceivedTimestamp := q.Start

	for {
		tailResponse := new(loghttp.TailResponse)
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
					conn, err = c.LiveTailQueryConn(ctx, q.QueryString, delayFor, q.Limit, lastReceivedTimestamp, q.Quiet)
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

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) || errors.Is(err, websocket.ErrCloseSent) {
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
