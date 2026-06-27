package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackoff(t *testing.T) {

	test := func(retryCount int, expected int) {
		require.Equal(t, time.Duration(expected)*time.Minute, backoff(retryCount))
	}

	test(0, 1)
	test(1, 2)
	test(2, 4)
	test(3, 8)
	test(4, 16)
	test(5, 32)
	test(6, 64)
	test(7, 128)
	test(8, 256)
	test(9, 512)
	test(10, 1024)
	test(11, 2048)
	test(12, 4096)
}

func TestBackoff_ClampPreventsOverflow(t *testing.T) {

	// At the clamp (27), the backoff must be a large positive duration. Without
	// the clamp, exponents >= 28 overflow time.Duration into negative values that
	// would schedule a retry in the past and hot-loop the queue.
	atClamp := backoff(maxBackoffExponent)
	require.Greater(t, atClamp, time.Duration(0))
	require.Equal(t, time.Duration(1<<maxBackoffExponent)*time.Minute, atClamp)

	// Every retry count past the clamp returns the same clamped value, never an
	// overflowed (negative or garbage) duration.
	for _, retryCount := range []int{28, 31, 42, 52, 63, 100, 1000} {
		result := backoff(retryCount)
		require.Equal(t, atClamp, result, "retryCount=%d must clamp to backoff(%d)", retryCount, maxBackoffExponent)
		require.Greater(t, result, time.Duration(0), "retryCount=%d must stay positive", retryCount)
	}

	// Just below the clamp is unaffected: backoff(26) = 2^26 minutes.
	require.Equal(t, time.Duration(1<<26)*time.Minute, backoff(26))
}
