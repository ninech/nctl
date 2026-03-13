// Package ipcheck provides a client for detecting the caller's public IP address.
package ipcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"time"
)

// ErrStatus represents a non success status code error.
var ErrStatus = errors.New("status code error")

const (
	// defaultTimeout is the default HTTP timeout.
	defaultTimeout = 5 * time.Second
	// defaultURL is the default endpoint to query.
	defaultURL = "https://ip-ban-check.nine.ch/"
)

// defaultClient returns the default Client instance.
var defaultClient = sync.OnceValue(func() *Client {
	return New()
})

// PublicIP returns the caller's public IP address as reported by the endpoint.
func PublicIP(ctx context.Context) (*Response, error) {
	return defaultClient().PublicIP(ctx)
}

// Client fetches the caller's public IP address from Nine's IP check endpoint.
type Client struct {
	// httpClient is the HTTP client to use. If nil, a default client with a 5s timeout is used.
	httpClient *http.Client
	// userAgent is the value to set in the User-Agent header.
	userAgent string
	// url is the endpoint to query. Defaults to https://ip-ban-check.nine.ch/.
	url *url.URL
}

// Response is the JSON response from the IP check endpoint.
type Response struct {
	Blocked    bool       `json:"blocked"`
	RemoteAddr netip.Addr `json:"remoteAddr"`
}

// Option is a function that configures a Client.
type Option func(*Client)

// WithHTTPClient configures the HTTP client to use.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithUserAgent configures the User-Agent header to use.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		c.userAgent = userAgent
	}
}

// WithURL configures the endpoint URL to query.
func WithURL(url *url.URL) Option {
	return func(c *Client) {
		c.url = url
	}
}

// New creates a new Client with the given options.
func New(options ...Option) *Client {
	u, _ := url.Parse(defaultURL)
	c := &Client{
		url:        u,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

// PublicIP returns the caller's public IP address as reported by the endpoint.
func (c *Client) PublicIP(ctx context.Context) (*Response, error) {
	req, err := c.newRequest(ctx, http.MethodGet)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	result := Response{}
	if _, err := c.doJSON(req, &result); err != nil {
		return nil, fmt.Errorf("decoding IP check response: %w", err)
	}

	return &result, nil
}

// newRequest creates a new HTTP request with the given method and URL.
func (c *Client) newRequest(ctx context.Context, method string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.url.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

// doJSON sends the given request and decodes the response into v.
func (c *Client) doJSON(req *http.Request, v any) (*http.Response, error) {
	resp, err := c.do(req)
	if err != nil {
		return resp, err
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	err = json.NewDecoder(resp.Body).Decode(&v)

	return resp, err
}

// do sends the given request and returns the response.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf(
			"%s: %d, %w",
			http.StatusText(resp.StatusCode),
			resp.StatusCode,
			ErrStatus,
		)
	}

	return resp, err
}
