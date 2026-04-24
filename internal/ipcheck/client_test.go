package ipcheck_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"

	"github.com/ninech/nctl/internal/ipcheck"
)

func TestClient_PublicIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		response   ipcheck.Response
		statusCode int
		wantIP     netip.Addr
		wantErr    bool
	}{
		{
			name:       "returns remote addr",
			response:   ipcheck.Response{Blocked: false, RemoteAddr: netip.MustParseAddr("203.0.113.1")},
			statusCode: http.StatusOK,
			wantIP:     netip.MustParseAddr("203.0.113.1"),
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept") != "application/json" {
					t.Errorf("expected Accept: application/json, got %q", r.Header.Get("Accept"))
				}
				w.WriteHeader(tc.statusCode)
				if tc.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tc.response)
				}
			}))
			defer srv.Close()

			srvURL, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatalf("parsing URL %q: %v", srv.URL, err)
			}

			c := ipcheck.New(
				ipcheck.WithURL(srvURL),
				ipcheck.WithHTTPClient(srv.Client()),
				ipcheck.WithUserAgent("nctl-test"),
			)

			got, err := c.PublicIP(t.Context())
			if (err != nil) != tc.wantErr {
				t.Fatalf("PublicIP() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got.RemoteAddr.Compare(tc.wantIP) != 0 {
				t.Errorf("PublicIP() = %q, want %q", got.RemoteAddr.String(), tc.wantIP)
			}
		})
	}
}
