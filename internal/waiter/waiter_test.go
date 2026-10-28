package waiter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestRetryWatch(t *testing.T) {
	t.Parallel()

	backoff := wait.Backoff{Steps: 3, Duration: time.Millisecond, Factor: 2, Cap: 5 * time.Millisecond}
	errOther := errors.New("other")
	dropped := watchError{kind: "test"}

	tests := []struct {
		name string
		// results are returned by the watch one after the other
		results   []watchResult
		wantErr   error
		wantCalls int
	}{
		{
			name:      "done",
			results:   []watchResult{{}},
			wantCalls: 1,
		},
		{
			name:      "otherError",
			results:   []watchResult{{err: errOther}},
			wantErr:   errOther,
			wantCalls: 1,
		},
		{
			// a watch which drops right away, e.g. after replaying the current
			// objects, does not refresh the budget
			name:      "quickDrops",
			results:   []watchResult{{err: dropped}, {err: dropped}, {err: dropped}, {}},
			wantErr:   dropped,
			wantCalls: 3,
		},
		{
			// a watch which stayed open refreshes the budget, so more drops
			// than steps are survived
			name: "stableDrops",
			results: []watchResult{
				{open: true, err: dropped}, {open: true, err: dropped},
				{open: true, err: dropped}, {open: true, err: dropped},
				{},
			},
			wantCalls: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			err := retryWatch(t.Context(), backoff, func() error {
				res := tt.results[calls]
				calls++
				if res.open {
					time.Sleep(2 * backoff.Cap)
				}
				return res.err
			})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantCalls, calls)
		})
	}
}

func TestRetryWatchContextDone(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// the delay is never waited for, the next watch reports how the wait ended
	backoff := wait.Backoff{Steps: 3, Duration: time.Hour, Cap: time.Hour}
	calls := 0
	err := retryWatch(ctx, backoff, func() error {
		calls++
		if calls > 1 {
			return nil
		}
		return watchError{kind: "test"}
	})

	require.NoError(t, err)
	require.Equal(t, 2, calls)
}

type watchResult struct {
	// open makes the watch stay open for longer than the cap of the backoff
	open bool
	err  error
}
