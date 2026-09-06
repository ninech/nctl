package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// makeJWT builds a signed JWT carrying the given exp claim. When exp is the
// zero value, no exp claim is set. The signature is irrelevant because the
// expiry check parses the token without verification.
func makeJWT(t *testing.T, exp time.Time) string {
	t.Helper()

	claims := jwt.RegisteredClaims{}
	if !exp.IsZero() {
		claims.ExpiresAt = jwt.NewNumericDate(exp)
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return token
}

func TestTokenExpiry(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)

	t.Run("reads exp claim", func(t *testing.T) {
		exp := now.Add(time.Hour)
		got, ok := tokenExpiry(makeJWT(t, exp))
		require.True(t, ok)
		require.True(t, got.Equal(exp), "got %s, want %s", got, exp)
	})

	t.Run("missing exp claim", func(t *testing.T) {
		_, ok := tokenExpiry(makeJWT(t, time.Time{}))
		require.False(t, ok)
	})

	t.Run("not a JWT", func(t *testing.T) {
		_, ok := tokenExpiry("not-a-jwt")
		require.False(t, ok)
	})
}

func TestAccessTokenExpired(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)

	for name, tc := range map[string]struct {
		token       string
		kubeExpiry  *time.Time
		wantExpired bool
		wantExpiry  time.Time
	}{
		"valid JWT with future exp": {
			token:       makeJWT(t, now.Add(time.Hour)),
			wantExpired: false,
			wantExpiry:  now.Add(time.Hour),
		},
		"JWT expired hours ago": {
			token:       makeJWT(t, now.Add(-3*time.Hour)),
			wantExpired: true,
			wantExpiry:  now.Add(-3 * time.Hour),
		},
		"JWT within expiry leeway is treated as expired": {
			token:       makeJWT(t, now.Add(tokenExpiryLeeway/2)),
			wantExpired: true,
			wantExpiry:  now.Add(tokenExpiryLeeway / 2),
		},
		"JWT exp takes precedence over a valid kubelogin expiry": {
			token:       makeJWT(t, now.Add(-time.Hour)),
			kubeExpiry:  new(now.Add(time.Hour)),
			wantExpired: true,
			wantExpiry:  now.Add(-time.Hour),
		},
		"missing exp falls back to expired kubelogin expiry": {
			token:       makeJWT(t, time.Time{}),
			kubeExpiry:  new(now.Add(-time.Hour)),
			wantExpired: true,
			wantExpiry:  now.Add(-time.Hour),
		},
		"missing exp falls back to valid kubelogin expiry": {
			token:       makeJWT(t, time.Time{}),
			kubeExpiry:  new(now.Add(time.Hour)),
			wantExpired: false,
			wantExpiry:  now.Add(time.Hour),
		},
		"missing exp and no kubelogin expiry is treated as not expired": {
			token:       makeJWT(t, time.Time{}),
			kubeExpiry:  nil,
			wantExpired: false,
		},
		"opaque token falls back to expired kubelogin expiry": {
			token:       "opaque-token",
			kubeExpiry:  new(now.Add(-time.Hour)),
			wantExpired: true,
			wantExpiry:  now.Add(-time.Hour),
		},
		"opaque token with no kubelogin expiry is treated as not expired": {
			token:       "opaque-token",
			kubeExpiry:  nil,
			wantExpired: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			expiry, expired := accessTokenExpired(tc.token, tc.kubeExpiry, now)
			require.Equal(t, tc.wantExpired, expired)
			if !tc.wantExpiry.IsZero() {
				require.True(t, expiry.Equal(tc.wantExpiry), "got %s, want %s", expiry, tc.wantExpiry)
			}
		})
	}
}

// TestValidTokenOrchestration exercises the force-refresh / no-stale-token
// logic of validToken without driving the real kubelogin flow. The fetch stub
// records how it was called and returns scripted tokens per attempt.
func TestValidTokenOrchestration(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	valid := makeJWT(t, now.Add(time.Hour))
	expired := makeJWT(t, now.Add(-time.Hour))

	for name, tc := range map[string]struct {
		// first/second are the tokens returned for forceRefresh=false / true.
		first, second   string
		secondErr       error
		wantToken       string
		wantErr         string
		wantForceSecond bool // expect a forced (forceRefresh=true) second fetch
	}{
		"fresh token is returned without a refresh": {
			first:           valid,
			wantToken:       valid,
			wantForceSecond: false,
		},
		"expired token triggers a forced refresh that succeeds": {
			first:           expired,
			second:          valid,
			wantToken:       valid,
			wantForceSecond: true,
		},
		"still expired after forced refresh errors instead of emitting it": {
			first:           expired,
			second:          expired,
			wantErr:         "access token expired on 2026-06-01T11:00:00Z",
			wantForceSecond: true,
		},
		"error from the forced refresh is propagated": {
			first:           expired,
			secondErr:       fmt.Errorf("boom"),
			wantErr:         "boom",
			wantForceSecond: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var calls []bool // forceRefresh per call
			fetch := func(_ context.Context, _, _ string, _, forceRefresh bool) (string, *time.Time, error) {
				calls = append(calls, forceRefresh)
				if forceRefresh {
					return tc.second, nil, tc.secondErr
				}
				return tc.first, nil, nil
			}

			token, err := validToken(context.Background(), "https://issuer", "client", false, now, fetch)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantToken, token)
			}

			if tc.wantForceSecond {
				require.Equal(t, []bool{false, true}, calls, "expected a normal fetch followed by a forced refresh")
			} else {
				require.Equal(t, []bool{false}, calls, "expected a single non-forced fetch")
			}
		})
	}
}
