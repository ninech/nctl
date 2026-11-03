package credential

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

// fakeKeyring is an in-memory keyring failing with err, if set,
// or blocking until unblock is closed, if set.
type fakeKeyring struct {
	mu      sync.Mutex
	entries map[string]string
	err     error
	unblock chan struct{}
}

func (k *fakeKeyring) wait() error {
	if k.unblock != nil {
		<-k.unblock
	}
	return k.err
}

func (k *fakeKeyring) Get(service, user string) (string, error) {
	if err := k.wait(); err != nil {
		return "", err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	secret, ok := k.entries[service+"|"+user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return secret, nil
}

func (k *fakeKeyring) Set(service, user, password string) error {
	if err := k.wait(); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.entries == nil {
		k.entries = map[string]string{}
	}
	k.entries[service+"|"+user] = password
	return nil
}

func (k *fakeKeyring) Delete(service, user string) error {
	if err := k.wait(); err != nil {
		return err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.entries[service+"|"+user]; !ok {
		return keyring.ErrNotFound
	}
	delete(k.entries, service+"|"+user)
	return nil
}

// newStore returns a store in a temporary directory using kr as keyring.
func newStore(t *testing.T, kr *fakeKeyring, opts ...Option) *Store {
	t.Helper()
	s := New(t.TempDir(), opts...)
	s.keyring = kr

	return s
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	s := New("dir")
	is.Equal("dir", s.dir)
	is.Equal(DefaultService, s.service)
	is.Equal(DefaultTimeout, s.timeout)
	is.False(s.insecure)
	is.Equal("key", s.account("key"))

	s = New("dir", WithService("svc"), WithNamespace("ns"), WithTimeout(time.Second), WithInsecure(true))
	is.Equal("svc", s.service)
	is.Equal(time.Second, s.timeout)
	is.True(s.insecure)
	is.Equal("ns/key", s.account("key"))
}

func TestSetUsesKeyring(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	kr := &fakeKeyring{}
	s := newStore(t, kr, WithService("svc"), WithNamespace("ns"))
	is.NoError(os.WriteFile(s.Path("key"), []byte("outdated"), 0o600))

	is.NoError(s.Set(t.Context(), "key", "secret"))
	is.NoFileExists(s.Path("key"), "a file would shadow the keyring")
	is.Equal("secret", kr.entries["svc|ns/key"])

	got, err := s.Get(t.Context(), "key")
	is.NoError(err)
	is.Equal("secret", got)
}

func TestSetFallsBackToFile(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	var fallbackPath string
	var fallbackCreated bool
	var fallbackErr error
	s := newStore(t, &fakeKeyring{err: errors.New("no keyring")}, WithFallbackHandler(func(path string, created bool, err error) {
		fallbackPath, fallbackCreated, fallbackErr = path, created, err
	}))
	s.dir = filepath.Join(s.dir, "nested")

	is.NoError(s.Set(t.Context(), "key", "secret"))
	is.Equal(s.Path("key"), fallbackPath)
	is.True(fallbackCreated, "the first fallback creates the file")
	is.EqualError(fallbackErr, "no keyring")

	is.NoError(s.Set(t.Context(), "key", "newer"))
	is.False(fallbackCreated, "a later fallback replaces the file")

	info, err := os.Stat(s.Path("key"))
	is.NoError(err)
	is.Equal(os.FileMode(0o600), info.Mode().Perm())
	info, err = os.Stat(s.dir)
	is.NoError(err)
	is.Equal(os.FileMode(0o700), info.Mode().Perm())

	got, err := s.Get(t.Context(), "key")
	is.NoError(err)
	is.Equal("newer", got)
}

func TestInsecureNeverTouchesKeyring(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	kr := &fakeKeyring{entries: map[string]string{DefaultService + "|key": "in-keyring"}}
	s := newStore(t, kr, WithInsecure(true), WithFallbackHandler(func(string, bool, error) {
		t.Error("an insecure store does not fall back")
	}))

	_, err := s.Get(t.Context(), "key")
	is.ErrorIs(err, ErrNotFound)

	is.NoError(s.Set(t.Context(), "key", "secret"))
	is.FileExists(s.Path("key"))
	is.Equal("in-keyring", kr.entries[DefaultService+"|key"])
}

func TestGetPrefersFile(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	s := newStore(t, &fakeKeyring{entries: map[string]string{DefaultService + "|key": "outdated"}})
	is.NoError(os.WriteFile(s.Path("key"), []byte("newest"), 0o600))

	got, err := s.Get(t.Context(), "key")
	is.NoError(err)
	is.Equal("newest", got)
}

func TestGetNotFound(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	_, err := newStore(t, &fakeKeyring{}).Get(t.Context(), "key")
	is.ErrorIs(err, ErrNotFound)
	is.ErrorIs(err, fs.ErrNotExist)

}

func TestGetUnavailable(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	_, err := newStore(t, &fakeKeyring{err: errors.New("no keyring")}).Get(t.Context(), "key")
	is.ErrorIs(err, ErrUnavailable)
	is.NotErrorIs(err, fs.ErrNotExist, "an unreachable keyring is not empty")
	is.ErrorContains(err, "no keyring")
}

func TestDelete(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	kr := &fakeKeyring{entries: map[string]string{DefaultService + "|key": "secret"}}
	s := newStore(t, kr)
	is.NoError(os.WriteFile(s.Path("key"), []byte("secret"), 0o600))

	is.NoError(s.Delete(t.Context(), "key"))
	is.NoFileExists(s.Path("key"))
	is.Empty(kr.entries)

	is.ErrorIs(s.Delete(t.Context(), "key"), ErrNotFound)
}

func TestDeleteWithoutKeyring(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	s := newStore(t, &fakeKeyring{err: errors.New("no keyring")})
	is.NoError(os.WriteFile(s.Path("key"), []byte("secret"), 0o600))

	is.NoError(s.Delete(t.Context(), "key"), "the file held the newest secret")
	is.NoFileExists(s.Path("key"))

	err := s.Delete(t.Context(), "key")
	is.ErrorIs(err, ErrUnavailable)
	is.NotErrorIs(err, fs.ErrNotExist)
}

func TestHangingKeyring(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	kr := &fakeKeyring{unblock: make(chan struct{})}
	t.Cleanup(func() { close(kr.unblock) })
	s := newStore(t, kr, WithTimeout(10*time.Millisecond))

	// A timed out write falls back to the file.
	is.NoError(s.Set(t.Context(), "key", "secret"))
	is.FileExists(s.Path("key"))

	is.NoError(os.Remove(s.Path("key")))
	_, err := s.Get(t.Context(), "key")
	is.ErrorIs(err, ErrUnavailable)
	is.ErrorContains(err, "keyring did not respond within 10ms")
}

func TestCancelledContext(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	kr := &fakeKeyring{unblock: make(chan struct{})}
	t.Cleanup(func() { close(kr.unblock) })
	s := newStore(t, kr)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	is.ErrorIs(s.Set(ctx, "key", "secret"), context.Canceled)
	is.NoFileExists(s.Path("key"), "a cancelled write does not fall back")
	is.ErrorIs(s.Delete(ctx, "key"), context.Canceled)
}
