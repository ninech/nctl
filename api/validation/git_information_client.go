package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api/util"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

type GitInformationClient struct {
	token        string
	url          *url.URL
	client       *http.Client
	logRetryFunc func(err error)
	retryBackoff wait.Backoff
}

// NewGitInformationClient returns a client which can be used to retrieve
// metadata information about a given git repository
func NewGitInformationClient(address string, token string) (*GitInformationClient, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("can not parse git information service URL: %w", err)
	}
	return defaultGitInformationClient(setURLDefaults(u), token), nil
}

func setURLDefaults(u *url.URL) *url.URL {
	// the git information service just responds to messages on /explore
	newURL := u.JoinPath("explore")
	if u.Scheme == "" {
		newURL.Scheme = "https"
	}
	return newURL
}

func defaultGitInformationClient(url *url.URL, token string) *GitInformationClient {
	g := &GitInformationClient{
		token:  token,
		url:    url,
		client: http.DefaultClient,
		retryBackoff: wait.Backoff{
			Steps:    5,
			Duration: 200 * time.Millisecond,
			Factor:   2.0,
			Jitter:   0.1,
			Cap:      2 * time.Second,
		},
	}
	g.logRetryFunc = func(err error) {
		g.logError("Retrying because of error: %v\n", err)
	}
	return g
}

func (g *GitInformationClient) logError(format string, v ...any) {
	fmt.Fprintf(os.Stderr, format, v...)
}

// SetLogRetryFunc allows to set the function which logs retries when doing
// requests to the git information service
func (g *GitInformationClient) SetLogRetryFunc(f func(err error)) {
	g.logRetryFunc = f
}

// SetRetryBackoffs sets the backoff properties for retries
func (g *GitInformationClient) SetRetryBackoffs(backoff wait.Backoff) {
	g.retryBackoff = backoff
}

func (g *GitInformationClient) repositoryInformation(ctx context.Context, url string, auth util.GitAuth) (*apps.GitExploreResponse, error) {
	req := apps.GitExploreRequest{
		Repository: url,
	}
	if auth.Enabled() {
		req.Auth = &apps.Auth{}
		if auth.HasBasicAuth() {
			req.Auth.BasicAuth = &apps.BasicAuth{
				Username: *auth.Username,
				Password: *auth.Password,
			}
		}
		if auth.HasPrivateKey() {
			req.Auth.PrivateKey = []byte(*auth.SSHPrivateKey)
		}
	}
	return g.sendRequest(ctx, req)
}

// RepositoryInformation returns information about a given git repository. It retries on client connection issues.
func (g *GitInformationClient) RepositoryInformation(ctx context.Context, url string, auth util.GitAuth) (*apps.GitExploreResponse, error) {
	var repoInfo *apps.GitExploreResponse
	err := retry.OnError(
		g.retryBackoff,
		func(err error) bool {
			if g.logRetryFunc != nil {
				g.logRetryFunc(err)
			}
			// retry regardless of the error
			return true
		},
		func() error {
			var err error
			repoInfo, err = g.repositoryInformation(ctx, url, auth)
			return err
		})
	return repoInfo, err
}

func (g *GitInformationClient) sendRequest(ctx context.Context, req apps.GitExploreRequest) (*apps.GitExploreResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("can not JSON marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, g.url.String(), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("can not create HTTP request: %w", err)
	}
	if g.token != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.token))
	}

	resp, err := g.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can not read response body: %w", err)
	}

	exploreResponse := &apps.GitExploreResponse{}
	if err := json.Unmarshal(body, exploreResponse); err != nil {
		return nil, fmt.Errorf(
			"can not unmarshal response %q with status code %d: %w",
			string(body),
			resp.StatusCode,
			err,
		)
	}
	return exploreResponse, nil
}

// containsTag returns true of the given tag exists in the git explore response
func containsTag(tag string, response *apps.GitExploreResponse) bool {
	if response.RepositoryInfo == nil {
		return false
	}
	for _, item := range response.RepositoryInfo.Tags {
		if item == tag {
			return true
		}
	}
	return false
}

// containsBranch returns true of the given branch exists in the git explore response
func containsBranch(branch string, response *apps.GitExploreResponse) bool {
	if response.RepositoryInfo == nil {
		return false
	}
	for _, item := range response.RepositoryInfo.Branches {
		if item == branch {
			return true
		}
	}
	return false
}

// RetryLogFunc returns a retry log function depending on the given showErrors
// parameter. If it is set to true, exact errors are shown when retrying to
// connect to the git information service. Otherwise they are not shown.
func RetryLogFunc(showErrors bool) func(err error) {
	return func(err error) {
		if showErrors {
			fmt.Fprintf(os.Stderr, "got error: %v\n", err)
		}
		fmt.Fprintln(os.Stderr, "Retrying...")
	}
}
