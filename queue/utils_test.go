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

// TestBackoff_NegativeRetryCountDoesNotPanic guards the lower end of the clamp
func TestBackoff_NegativeRetryCountDoesNotPanic(t *testing.T) {

	// A variable shift by a negative amount panics at run time, so a corrupt record with a
	// negative RetryCount must be treated as the first attempt rather than crashing a worker.

	for _, retryCount := range []int{-1, -8, -1000} {
		require.NotPanics(t, func() { backoff(retryCount) }, "retryCount=%d", retryCount)
		require.Equal(t, 1*time.Minute, backoff(retryCount), "retryCount=%d", retryCount)
	}
}

// TestRetryDelay_Uncapped confirms that a task with no BackoffMax keeps the doubling schedule
func TestRetryDelay_Uncapped(t *testing.T) {

	for _, retryCount := range []int{0, 1, 7, 11, 20} {
		require.Equal(t, backoff(retryCount), retryDelay(Task{RetryCount: retryCount}), "retryCount=%d", retryCount)
	}

	// A negative cap is a corrupt record, and must not turn every wait into zero
	require.Equal(t, backoff(3), retryDelay(Task{RetryCount: 3, BackoffMax: -60}))
}

// TestRetryDelay_Capped confirms that the wait doubles up to BackoffMax and then stays there
func TestRetryDelay_Capped(t *testing.T) {

	day := int64((24 * time.Hour) / time.Second)

	test := func(retryCount int, expected time.Duration) {
		require.Equal(t, expected, retryDelay(Task{RetryCount: retryCount, BackoffMax: day}), "retryCount=%d", retryCount)
	}

	test(0, time.Minute)
	test(10, 1024*time.Minute)
	test(11, 24*time.Hour)
	test(16, 24*time.Hour)
	test(1000, 24*time.Hour)
}

// TestRetryDelay_SevenDaySchedule pins the delivery schedule that the cap exists for: a task
// with seventeen retries and a one-day cap is still retrying after seven days
func TestRetryDelay_SevenDaySchedule(t *testing.T) {

	task := Task{RetryMax: 17, BackoffMax: int64((24 * time.Hour) / time.Second)}
	total := time.Duration(0)

	for task.RetryCount = 0; task.RetryCount < task.RetryMax; task.RetryCount++ {
		total += retryDelay(task)
	}

	require.GreaterOrEqual(t, total, 7*24*time.Hour)
	require.Less(t, total, 8*24*time.Hour)
}
