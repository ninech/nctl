package test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
)

// GitOnceUploadResponse is the response returned by the gitonce upload service.
type GitOnceUploadResponse struct {
	Message string `json:"message"`
	URL     string `json:"url"`
	Commit  string `json:"commit"`
}

type gitOnceService struct {
	sync.Mutex
	server   *httptest.Server
	logger   *slog.Logger
	response GitOnceUploadResponse
	// receivedZip holds the raw bytes of the last uploaded zip file.
	receivedZip []byte
}

// NewGitOnceService returns a mock gitonce upload service for use in tests.
func NewGitOnceService() *gitOnceService {
	g := &gitOnceService{
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		response: GitOnceUploadResponse{
			Message: "upload successful",
			URL:     "https://gitonce.example.com/gitonce/test-repo.git",
		},
	}
	mux := http.NewServeMux()
	mux.Handle("/upload", g)
	g.server = httptest.NewUnstartedServer(mux)
	return g
}

// SetResponse sets the response returned by the mock service.
func (g *gitOnceService) SetResponse(r GitOnceUploadResponse) {
	g.Lock()
	defer g.Unlock()
	g.response = r
}

// ReceivedZip returns the raw bytes of the last uploaded zip.
func (g *gitOnceService) ReceivedZip() []byte {
	g.Lock()
	defer g.Unlock()
	return g.receivedZip
}

func (g *gitOnceService) Start() {
	g.server.Start()
}

func (g *gitOnceService) Close() {
	if g.server != nil {
		g.server.Close()
	}
}

// URL returns the base URL of the mock server (without /upload path).
func (g *gitOnceService) URL() string {
	return g.server.URL + "/upload"
}

func (g *gitOnceService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad multipart request", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("zipfile")
	if err != nil {
		http.Error(w, "missing zipfile field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data := make([]byte, 0, 1024)
	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		data = append(data, buf[:n]...)
		if err != nil {
			break
		}
	}

	g.Lock()
	g.receivedZip = data
	resp := g.response
	g.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		g.logger.Error("error encoding response", "error", err.Error())
	}
}