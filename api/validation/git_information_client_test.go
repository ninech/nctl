package validation_test

import (
	"net/http"
	"testing"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/api/validation"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
)

func TestRepositoryInformation(t *testing.T) {
	t.Parallel()

	gitInfo := test.NewGitInformationService()
	gitInfo.Start()
	defer gitInfo.Close()

	is := require.New(t)
	dummyPrivateKey, err := test.GenerateRSAPrivateKey()
	is.NoError(err)

	for name, testCase := range map[string]struct {
		git              apps.GitTarget
		token            string
		auth             util.GitAuth
		verifyRequest    func(t *testing.T) func(p test.GitInfoServiceParsed, err error)
		setResponse      *test.GitInformationServiceResponse
		expectedResponse *apps.GitExploreResponse
		expectedRetries  int
		backoff          *wait.Backoff
		errorExpected    bool
	}{
		"validate request": {
			git: apps.GitTarget{
				URL:      "https://github.com/ninech/deploio-examples",
				Revision: "main",
			},
			token: "fake",
			auth: util.GitAuth{
				Username:      ptr.To("fake"),
				Password:      ptr.To("fakePass"),
				SSHPrivateKey: &dummyPrivateKey,
			},
			setResponse: &test.GitInformationServiceResponse{
				Code: http.StatusOK,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      "https://github.com/ninech/deploio-examples",
						Branches: []string{"main"},
						Tags:     []string{"v1.0"},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: "main",
							Found:             true,
						},
					},
				},
			},
			expectedResponse: &apps.GitExploreResponse{
				RepositoryInfo: &apps.RepositoryInfo{
					URL:      "https://github.com/ninech/deploio-examples",
					Branches: []string{"main"},
					Tags:     []string{"v1.0"},
					RevisionResponse: &apps.RevisionResponse{
						RevisionRequested: "main",
						Found:             true,
					},
				},
			},
			verifyRequest: func(t *testing.T) func(p test.GitInfoServiceParsed, err error) {
				return func(p test.GitInfoServiceParsed, err error) {
					is := require.New(t)
					is.NoError(err)
					is.Equal("https://github.com/ninech/deploio-examples", p.Request.Repository)
					is.Equal("fake", p.Token)
					is.Equal(http.MethodPost, p.Method)
					is.NotNil(p.Request.Auth)
					is.NotNil(p.Request.Auth.BasicAuth)
					is.Equal("fake", p.Request.Auth.BasicAuth.Username)
					is.Equal("fakePass", p.Request.Auth.BasicAuth.Password)
					is.Equal(dummyPrivateKey, string(p.Request.Auth.PrivateKey))
				}
			},
		},
		"we retry on server errors": {
			git: apps.GitTarget{
				URL:      "https://github.com/ninech/deploio-examples",
				Revision: "main",
			},
			token:           "fake",
			expectedRetries: 2,
			backoff: &wait.Backoff{
				Duration: 100 * time.Millisecond,
				Steps:    2,
			},
			setResponse: &test.GitInformationServiceResponse{
				Code: http.StatusBadGateway,
				Raw:  ptr.To("currently unavailable"),
			},
			errorExpected: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			is := require.New(t)

			if testCase.setResponse != nil {
				gitInfo.SetResponse(*testCase.setResponse)
			}

			c, err := validation.NewGitInformationClient(gitInfo.URL(), testCase.token)
			is.NoError(err)

			// we count the retries of the request
			retries := 0
			c.SetLogRetryFunc(func(_ error) {
				retries++
			})
			if testCase.backoff != nil {
				c.SetRetryBackoffs(*testCase.backoff)
			}

			response, err := c.RepositoryInformation(t.Context(), testCase.git, testCase.auth)
			if testCase.errorExpected {
				is.Error(err)
			} else {
				is.NoError(err)
			}

			is.Equal(testCase.expectedRetries, retries)
			if testCase.expectedResponse != nil {
				is.Equal(*testCase.expectedResponse, *response)
			}
			if testCase.verifyRequest != nil {
				data, err := gitInfo.Request()
				testCase.verifyRequest(t)(data, err)
			}
		})
	}
}
