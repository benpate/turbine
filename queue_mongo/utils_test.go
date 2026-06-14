package queue_mongo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeoutContext(t *testing.T) {

	ctx, cancel := timeoutContext(16)
	defer cancel()

	// The context has a deadline roughly 16 seconds in the future
	deadline, ok := ctx.Deadline()
	require.True(t, ok)

	remaining := time.Until(deadline)
	require.Greater(t, remaining, 14*time.Second)
	require.LessOrEqual(t, remaining, 16*time.Second)
}

func TestTimeoutContext_Cancel(t *testing.T) {

	ctx, cancel := timeoutContext(30)

	// Cancelling the context closes its Done channel with a Cancelled error
	cancel()

	select {
	case <-ctx.Done():
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("expected context to be cancelled")
	}
}

func TestTimeoutContext_Zero(t *testing.T) {

	// A zero timeout produces an already-expired deadline
	ctx, cancel := timeoutContext(0)
	defer cancel()

	_, ok := ctx.Deadline()
	require.True(t, ok)
}
