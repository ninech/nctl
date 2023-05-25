package log

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grafana/loki/pkg/loghttp"
	legacy "github.com/grafana/loki/pkg/loghttp/legacy"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/grafana/loki/pkg/util"
	"github.com/grafana/loki/pkg/util/marshal"
)

type fake struct {
	timestamp time.Time
	lines     []string
	wsAddr    string
}

func NewFake(t *testing.T, expectedTime time.Time, expectedLines ...string) *fake {
	s := httptest.NewServer(lokiTailHandler(t, expectedTime, expectedLines))
	t.Cleanup(func() { s.Close() })

	return &fake{timestamp: expectedTime, lines: expectedLines, wsAddr: "ws" + strings.TrimPrefix(s.URL, "http")}
}

func lokiTailHandler(t *testing.T, timestamp time.Time, lines []string) http.HandlerFunc {
	upgrader := websocket.Upgrader{}
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := loghttp.ParseTailQuery(r)
		if err != nil {
			t.Error(err)
			return
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close()

		entries := []logproto.Entry{}
		for v, line := range lines {
			if v == int(req.Limit) {
				break
			}
			entries = append(entries, logproto.Entry{
				Timestamp: timestamp,
				Line:      line,
			})
		}
		resp := legacy.TailResponse{
			Streams: []logproto.Stream{
				{
					Labels:  "ab",
					Entries: entries,
				},
			},
		}

		if err := marshal.WriteTailResponseJSON(resp, c); err != nil {
			t.Error(err)
			return
		}

		cm := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closed")
		if err := c.WriteMessage(websocket.CloseMessage, cm); err != nil {
			t.Error(err)
			return
		}
	}
}
func (f fake) QueryRange(queryStr string, limit int, start, end time.Time, direction logproto.Direction, step, interval time.Duration, quiet bool) (*loghttp.QueryResponse, error) {
	entries := []loghttp.Entry{}
	for v, line := range f.lines {
		if v == limit {
			break
		}
		entries = append(entries, loghttp.Entry{
			Timestamp: f.timestamp,
			Line:      line,
		})
	}

	return &loghttp.QueryResponse{
		Data: loghttp.QueryResponseData{
			ResultType: loghttp.ResultTypeStream,
			Result: loghttp.Streams{
				{
					Labels:  loghttp.LabelSet{"a": "b"},
					Entries: entries,
				},
			},
		},
	}, nil
}

func (f fake) LiveTailQueryConn(queryStr string, delayFor time.Duration, limit int, start time.Time, quiet bool) (*websocket.Conn, error) {
	u, err := url.Parse(f.wsAddr)
	if err != nil {
		return nil, err
	}

	params := util.NewQueryStringBuilder()
	params.SetString("query", queryStr)
	if delayFor != 0 {
		params.SetInt("delay_for", int64(delayFor.Seconds()))
	}
	params.SetInt("limit", int64(limit))
	params.SetInt("start", start.UnixNano())
	u.RawQuery = params.Encode()

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	return ws, nil
}

func (f fake) ListLabelNames(quiet bool, start, end time.Time) (*loghttp.LabelResponse, error) {
	return nil, nil
}

func (f fake) ListLabelValues(name string, quiet bool, start, end time.Time) (*loghttp.LabelResponse, error) {
	return nil, nil
}

func (f fake) Series(matchers []string, start, end time.Time, quiet bool) (*loghttp.SeriesResponse, error) {
	return nil, nil
}

func (f fake) Query(queryStr string, limit int, time time.Time, direction logproto.Direction, quiet bool) (*loghttp.QueryResponse, error) {
	return nil, nil
}

func (f fake) GetOrgID() string {
	return "fake"
}
