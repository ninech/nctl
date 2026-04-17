package gitonce

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// initGitRepo initialises a minimal git repo in dir, adds the given files, and
// commits them so that git ls-files returns them.
func initGitRepo(t *testing.T, dir string, tracked []string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	for _, f := range tracked {
		run("add", f)
	}
	run("commit", "-m", "init")
}

func zipNames(t *testing.T, buf *bytes.Buffer) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	m := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		rc.Close()
		require.NoError(t, err)
		m[f.Name] = string(content)
	}
	return m
}

// TestFilesToZipGitRepo verifies that only git-tracked files are included when
// the directory is a git repository.
func TestFilesToZipGitRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "helper.go"), []byte("package sub"), 0600))
	// untracked file – must not appear in the zip
	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.go"), []byte("package untracked"), 0600))

	initGitRepo(t, dir, []string{"main.go", filepath.Join("sub", "helper.go")})

	buf, err := zipDirectory(dir)
	require.NoError(t, err)

	names := zipNames(t, buf)

	require.Equal(t, "package main", names["main.go"])
	require.Equal(t, "package sub", names[filepath.Join("sub", "helper.go")])
	require.NotContains(t, names, "untracked.go", "untracked files must be excluded")
	for name := range names {
		require.NotContains(t, name, ".git", ".git internals must be excluded")
	}
}

// TestFilesToZipGitignore verifies that .gitignore patterns are respected when
// the directory is not a git repository.
func TestFilesToZipGitignore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	gitignore := "*.log\nbuild/\n# comment\n!important.log\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "debug.log"), []byte("log data"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "important.log"), []byte("important"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "build"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build", "output"), []byte("binary"), 0600))

	buf, err := zipDirectory(dir)
	require.NoError(t, err)

	names := zipNames(t, buf)

	require.Contains(t, names, "main.go")
	require.Contains(t, names, ".gitignore")
	require.Contains(t, names, "important.log", "negated pattern must be included")
	require.NotContains(t, names, "debug.log", "*.log must be excluded")
	require.NotContains(t, names, filepath.Join("build", "output"), "build/ directory must be excluded")
}

// TestFilesToZipPlain verifies that all files are included when there is
// neither a .git directory nor a .gitignore.
func TestFilesToZipPlain(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "b", "file.txt"), []byte("hello"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root"), 0600))

	buf, err := zipDirectory(dir)
	require.NoError(t, err)

	names := zipNames(t, buf)
	var got []string
	for n := range names {
		got = append(got, n)
	}
	sort.Strings(got)
	require.Equal(t, []string{filepath.Join("a", "b", "file.txt"), "root.txt"}, got)
}

func TestZipDirectoryContentsAreRelative(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "b", "file.txt"), []byte("hello"), 0600))

	buf, err := zipDirectory(dir)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	require.Len(t, zr.File, 1)
	// path must be relative, not contain the temp dir prefix
	require.Equal(t, filepath.Join("a", "b", "file.txt"), zr.File[0].Name)
}

func TestUploadDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.go"), []byte("package app"), 0600))

	t.Run("successful upload returns git URL", func(t *testing.T) {
		t.Parallel()

		expectedURL := "https://gitonce.example.com/gitonce/1234-abcd.git"
		expectedCommit := "b208780317e1726aa6024368fa16f3a74ff5299d"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)

			if err := r.ParseMultipartForm(10 << 20); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			f, _, err := r.FormFile("zipfile")
			if err != nil {
				http.Error(w, "missing zipfile", http.StatusBadRequest)
				return
			}
			defer f.Close()

			// verify the uploaded file is a valid zip
			data, err := io.ReadAll(f)
			require.NoError(t, err)
			_, err = zip.NewReader(bytes.NewReader(data), int64(len(data)))
			require.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(uploadResponse{
				Message: "upload successful",
				URL:     expectedURL,
				Commit:  expectedCommit,
			})
		}))
		defer srv.Close()

		result, err := UploadDirectory(context.Background(), dir, srv.URL)
		require.NoError(t, err)
		require.Equal(t, expectedURL, result.URL)
		require.Equal(t, expectedCommit, result.Commit)
	})

	t.Run("non-200 status returns error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := UploadDirectory(context.Background(), dir, srv.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "500")
	})

	t.Run("empty URL in response returns error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(uploadResponse{Message: "upload successful", URL: ""})
		}))
		defer srv.Close()

		_, err := UploadDirectory(context.Background(), dir, srv.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty URL")
	})

	t.Run("invalid JSON response returns error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))
		defer srv.Close()

		_, err := UploadDirectory(context.Background(), dir, srv.URL)
		require.Error(t, err)
	})

	t.Run("non-existent directory returns error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		_, err := UploadDirectory(context.Background(), "/nonexistent/path/to/dir", srv.URL)
		require.Error(t, err)
	})

	t.Run("context cancellation is respected", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := UploadDirectory(ctx, dir, srv.URL)
		require.Error(t, err)
	})
}