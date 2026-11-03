// Package credential stores secrets in the keyring of the operating system
// and falls back to files only the user can read where the keyring is unavailable.
package credential

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	// DefaultService is the keyring service of a [Store] without [WithService].
	DefaultService = "nctl"
	// DefaultTimeout bounds a keyring operation of a [Store] without [WithTimeout].
	DefaultTimeout = 10 * time.Second
)

var (
	// ErrNotFound is returned when a store holds no secret for a key.
	// It matches [fs.ErrNotExist].
	ErrNotFound = fmt.Errorf("credential not found: %w", fs.ErrNotExist)
	// ErrUnavailable is returned when the keyring cannot be reached,
	// so the store cannot tell whether it holds a secret.
	ErrUnavailable = errors.New("keyring unavailable")
)

// backend is the keyring of the operating system, replaced in tests.
type backend interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

// systemKeyring is the keyring of the operating system.
type systemKeyring struct{}

func (systemKeyring) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (systemKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}
func (systemKeyring) Delete(service, user string) error { return keyring.Delete(service, user) }

// Store keeps secrets by key in the keyring of the operating system
// or, where the keyring is unavailable, in files below a directory.
//
// A file exists only while it holds the newest secret of its key,
// as writing a secret to the keyring removes its file.
// Reads therefore prefer the file,
// which also picks up secrets written before the keyring was used.
type Store struct {
	dir        string
	service    string
	namespace  string
	timeout    time.Duration
	insecure   bool
	onFallback FallbackHandler
	keyring    backend
}

// Option configures a [Store].
type Option func(*Store)

// WithService sets the keyring service holding the secrets,
// [DefaultService] by default.
func WithService(service string) Option {
	return func(s *Store) {
		s.service = service
	}
}

// WithNamespace prefixes the names of the keyring entries,
// so stores of different kinds of secrets can share a service.
func WithNamespace(namespace string) Option {
	return func(s *Store) {
		s.namespace = namespace
	}
}

// WithTimeout bounds each keyring operation,
// which can hang while the keyring waits for an unlock prompt nobody answers.
// It is [DefaultTimeout] by default.
func WithTimeout(timeout time.Duration) Option {
	return func(s *Store) {
		s.timeout = timeout
	}
}

// WithInsecure makes the store use files only and never touch the keyring.
func WithInsecure(insecure bool) Option {
	return func(s *Store) {
		s.insecure = insecure
	}
}

// FallbackHandler is called with the path of the file and the error of the keyring
// when a secret is written to a file because the keyring failed.
// created is true if the file did not exist before,
// so the secret of the key is in plain text for the first time.
type FallbackHandler func(path string, created bool, err error)

// WithFallbackHandler sets the handler called whenever a secret is written to a file
// because the keyring failed.
// It is not called for a store using [WithInsecure].
func WithFallbackHandler(handler FallbackHandler) Option {
	return func(s *Store) {
		s.onFallback = handler
	}
}

// New returns a store keeping the files of its secrets in dir.
func New(dir string, opts ...Option) *Store {
	s := &Store{
		dir:     dir,
		service: DefaultService,
		timeout: DefaultTimeout,
		keyring: systemKeyring{},
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Path returns the file holding the secret of key when it is not in the keyring.
// key must be a valid file name.
func (s *Store) Path(key string) string {
	return filepath.Join(s.dir, key)
}

// Get returns the secret of key.
// It returns [ErrNotFound] if neither the file nor the keyring holds it,
// and an error wrapping [ErrUnavailable] if there is no file and the keyring cannot be reached.
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	b, err := os.ReadFile(s.Path(key))
	if err == nil {
		return string(b), nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if s.insecure {
		return "", ErrNotFound
	}

	var secret string
	err = call(ctx, s.timeout, func() (err error) {
		secret, err = s.keyring.Get(s.service, s.account(key))
		return err
	})
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", keyringError(ctx, "reading", err)
	}

	return secret, nil
}

// Set stores secret under key.
// It writes to the keyring unless the store is insecure,
// and falls back to a file if the keyring fails.
func (s *Store) Set(ctx context.Context, key, secret string) error {
	var keyringErr error
	if !s.insecure {
		keyringErr = call(ctx, s.timeout, func() error {
			return s.keyring.Set(s.service, s.account(key), secret)
		})
		if keyringErr == nil {
			// A remaining file would shadow the secret in the keyring.
			if err := os.Remove(s.Path(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("removing outdated credential file: %w", err)
			}

			return nil
		}
		if ctx.Err() != nil {
			return keyringErr
		}
	}

	_, statErr := os.Stat(s.Path(key))
	created := errors.Is(statErr, fs.ErrNotExist)
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("creating credential directory: %w", err)
	}
	if err := os.WriteFile(s.Path(key), []byte(secret), 0o600); err != nil {
		return fmt.Errorf("writing credential file: %w", err)
	}
	if keyringErr != nil && s.onFallback != nil {
		s.onFallback(s.Path(key), created, keyringErr)
	}

	return nil
}

// Delete removes the secret of key from both the file and the keyring.
// It returns [ErrNotFound] if neither held it.
// If the keyring cannot be reached, Delete succeeds when it removed the file,
// which held the newest secret,
// and otherwise returns an error wrapping [ErrUnavailable].
func (s *Store) Delete(ctx context.Context, key string) error {
	err := os.Remove(s.Path(key))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing credential file: %w", err)
	}
	removedFile := err == nil
	if s.insecure {
		if !removedFile {
			return ErrNotFound
		}
		return nil
	}

	err = call(ctx, s.timeout, func() error {
		return s.keyring.Delete(s.service, s.account(key))
	})
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return ctx.Err()
	case removedFile:
		return nil
	case errors.Is(err, keyring.ErrNotFound):
		return ErrNotFound
	default:
		return keyringError(ctx, "deleting from", err)
	}
}

// keyringError returns the error of a failed keyring operation:
// the error of ctx if it is done, or else err wrapped in [ErrUnavailable].
func keyringError(ctx context.Context, op string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return fmt.Errorf("%w: %s the keyring: %w", ErrUnavailable, op, err)
}

// account returns the name of the keyring entry of key.
func (s *Store) account(key string) string {
	if s.namespace == "" {
		return key
	}

	return s.namespace + "/" + key
}

// call runs the keyring operation f until it returns, ctx is done or timeout passes.
// The keyring does not support cancellation,
// so an abandoned operation keeps running in the background until it returns.
// f must therefore not write to variables the caller reads after call returned an error.
func call(ctx context.Context, timeout time.Duration, f func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- f()
	}()

	ctx, cancel := context.WithTimeoutCause(ctx, timeout, fmt.Errorf("keyring did not respond within %s", timeout))
	defer cancel()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}
