// Package gitonce provides a client to upload local directories to the gitonce
// service, which converts them to one-time-use git repositories.
package gitonce

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const DefaultUploadURL = "https://gitonce.21ddead.deploio.app/upload"

type uploadResponse struct {
	Message string `json:"message"`
	URL     string `json:"url"`
	Commit  string `json:"commit"`
}

// UploadResult holds the result of a successful gitonce upload.
type UploadResult struct {
	// URL is the one-time-use git repository URL.
	URL string
	// Commit is the commit hash of the uploaded content.
	Commit string
}

// UploadDirectory zips the given local directory and uploads it to the gitonce
// service at uploadURL. It returns the upload result containing the URL and
// commit hash.
func UploadDirectory(ctx context.Context, dir, uploadURL string) (UploadResult, error) {
	buf, err := zipDirectory(dir)
	if err != nil {
		return UploadResult{}, fmt.Errorf("zipping directory %q: %w", dir, err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("zipfile", "source.zip")
	if err != nil {
		return UploadResult{}, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(fw, buf); err != nil {
		return UploadResult{}, fmt.Errorf("writing zip to form: %w", err)
	}
	if err := mw.Close(); err != nil {
		return UploadResult{}, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("creating upload request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("uploading to gitonce: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return UploadResult{}, fmt.Errorf("gitonce upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var uploadResp uploadResponse
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return UploadResult{}, fmt.Errorf("decoding gitonce response: %w", err)
	}

	if uploadResp.URL == "" {
		return UploadResult{}, fmt.Errorf("gitonce returned empty URL")
	}

	return UploadResult{URL: uploadResp.URL, Commit: uploadResp.Commit}, nil
}

func zipDirectory(dir string) (*bytes.Buffer, error) {
	files, err := filesToZip(dir)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	for _, rel := range files {
		f, err := zw.Create(rel)
		if err != nil {
			return nil, err
		}
		src, err := os.Open(filepath.Join(dir, rel))
		if err != nil {
			return nil, err
		}
		_, cpErr := io.Copy(f, src)
		src.Close()
		if cpErr != nil {
			return nil, cpErr
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf, nil
}

// filesToZip returns the list of relative file paths to include in the zip.
// If dir is a git repository, only tracked files are included.
// Otherwise, all files are included while respecting .gitignore if present.
func filesToZip(dir string) ([]string, error) {
	if isGitRepo(dir) {
		return gitTrackedFiles(dir)
	}
	return walkWithGitignore(dir)
}

// isGitRepo reports whether dir is inside a git repository.
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// gitTrackedFiles returns relative paths of files tracked by git in dir.
func gitTrackedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running git ls-files: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// walkWithGitignore walks dir and returns relative file paths, excluding files
// matched by .gitignore patterns if a .gitignore file is present at dir root.
func walkWithGitignore(dir string) ([]string, error) {
	patterns, err := loadGitignorePatterns(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if rel == "." {
				return nil
			}
			if matchesGitignore(patterns, rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if !matchesGitignore(patterns, rel, false) {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

// gitignorePattern holds a parsed .gitignore rule.
type gitignorePattern struct {
	pattern  string
	negate   bool
	dirOnly  bool
	anchored bool // pattern contains a slash (other than trailing)
}

func loadGitignorePatterns(path string) ([]gitignorePattern, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []gitignorePattern
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := gitignorePattern{}
		if strings.HasPrefix(line, "!") {
			p.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		// A pattern is anchored if it contains a slash after stripping a
		// possible leading slash.
		trimmed := strings.TrimPrefix(line, "/")
		p.anchored = strings.Contains(trimmed, "/")
		p.pattern = trimmed
		patterns = append(patterns, p)
	}
	return patterns, sc.Err()
}

// matchesGitignore reports whether rel (a slash-separated relative path) is
// matched (i.e. should be ignored) by the given patterns.
func matchesGitignore(patterns []gitignorePattern, rel string, isDir bool) bool {
	rel = filepath.ToSlash(rel)
	ignored := false
	for _, p := range patterns {
		if p.dirOnly && !isDir {
			continue
		}
		var matched bool
		if p.anchored {
			matched, _ = filepath.Match(p.pattern, rel)
		} else {
			// match against the base name or any path component
			base := filepath.Base(rel)
			matched, _ = filepath.Match(p.pattern, base)
			if !matched {
				matched, _ = filepath.Match(p.pattern, rel)
			}
		}
		if matched {
			ignored = !p.negate
		}
	}
	return ignored
}