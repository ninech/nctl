package test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"

	apps "github.com/ninech/apis/apps/v1alpha1"
)

// GitInformationServiceResponse describes a response of the git information service
type GitInformationServiceResponse struct {
	// Code is the status code to be set
	Code int
	// Content describes the GitExploreResponse to be returned.
	Content apps.GitExploreResponse
	// Raw allows to return any text instead of a real GitExploreResponse.
	// If it is set, it has precedence over the Content field.
	Raw *string
}

// GitInfoServiceParsed represents are parsed request received by the git
// information service
type GitInfoServiceParsed struct {
	Token   string
	Method  string
	Request apps.GitExploreRequest
}

// VerifyRequestFunc can be used to verify the parsed request which was sent to
// the git information service
type VerifyRequestFunc func(p GitInfoServiceParsed, err error)

type gitInformationService struct {
	sync.Mutex
	server   *httptest.Server
	logger   *slog.Logger
	response GitInformationServiceResponse
	request  struct {
		data GitInfoServiceParsed
		err  error
	}
}

func defaultResponse() GitInformationServiceResponse {
	return GitInformationServiceResponse{
		Code: 404,
		Raw:  new("no response set"),
	}
}

// NewGitInformationService returns a new git information service mock. It can
// be used to verify requests sent to it and also to just return with a
// previously set response.
func NewGitInformationService() *gitInformationService {
	g := &gitInformationService{
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		response: defaultResponse(),
	}
	mux := http.NewServeMux()
	mux.Handle("/explore", g)
	g.server = httptest.NewUnstartedServer(mux)
	return g
}

// SetResponse sets the response to be returned
func (g *gitInformationService) SetResponse(r GitInformationServiceResponse) {
	g.Lock()
	defer g.Unlock()
	g.response = r
}

func (g *gitInformationService) Start() {
	g.server.Start()
}

func (g *gitInformationService) Close() {
	if g.server != nil {
		g.server.Close()
	}
}

func (g *gitInformationService) URL() string {
	return g.server.URL
}

// Request returns the parsed last request to the service or an eventual error which occurred during parsing
func (g *gitInformationService) Request() (GitInfoServiceParsed, error) {
	return g.request.data, g.request.err
}

func (g *gitInformationService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.request.data, g.request.err = parseRequest(r)

	w.WriteHeader(g.response.Code)
	if g.response.Raw != nil {
		_, err := w.Write([]byte(*g.response.Raw))
		if err != nil {
			g.logger.Error(err.Error())
		}
		return
	}
	content, err := json.Marshal(g.response.Content)
	if err != nil {
		g.logger.Error("error when marshaling response", "error", err.Error())
	}
	_, err = w.Write(content)
	if err != nil {
		g.logger.Error(err.Error())
	}
}

func parseRequest(r *http.Request) (GitInfoServiceParsed, error) {
	p := GitInfoServiceParsed{}
	p.Token = r.Header.Get("Authorization")
	if strings.HasPrefix(p.Token, "Bearer") {
		p.Token = strings.TrimSpace(p.Token[len("Bearer"):])
	}

	exploreRequest := apps.GitExploreRequest{}
	if err := unmarshalRequest(r.Body, &exploreRequest); err != nil {
		return p, err
	}
	p.Request = exploreRequest
	p.Method = r.Method
	return p, nil
}

func unmarshalRequest(data io.Reader, request *apps.GitExploreRequest) error {
	decoder := json.NewDecoder(data)
	err := decoder.Decode(request)
	if err != nil {
		return err
	}
	_, err = decoder.Token()
	if err == nil || !errors.Is(err, io.EOF) {
		return errors.New("invalid data after top-level JSON value")
	}
	return nil
}
