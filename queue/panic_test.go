package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestConsumeSafely_ContainsPanic confirms that a panicking Consumer does not unwind into the
// worker goroutine.  Without the recover, this panic reaches the runtime and kills the process,
// taking every other in-flight Task with it.
func TestConsumeSafely_ContainsPanic(t *testing.T) {

	q := New(WithStorage(&mockStorage{}), withConsumerFuncs(func(Task) Result {
		panic("consumer exploded")
	}))

	require.NotPanics(t, func() {
		q.consumeSafely(Task{TaskID: "abc", Name: "x", RetryMax: 8})
	})
}

// TestConsumeSafely_PanicIsRetriedThenAbandoned confirms that a panicking Task is routed through
// the normal error path.  Dropping it instead would leave its storage record in place, to be
// re-locked and re-panicked once the lease expired -- forever.
func TestConsumeSafely_PanicIsRetriedThenAbandoned(t *testing.T) {

	t.Run("retried while attempts remain", func(t *testing.T) {

		storage := &mockStorage{}
		q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
			panic("consumer exploded")
		}))

		q.consumeSafely(Task{TaskID: "abc", Name: "x", RetryCount: 0, RetryMax: 8})

		require.Len(t, storage.saved, 1, "the task is written back for another attempt")
		require.Equal(t, 1, storage.saved[0].RetryCount, "and its failure is counted")
		require.Empty(t, storage.failures)
	})

	t.Run("abandoned on the final attempt", func(t *testing.T) {

		storage := &mockStorage{}
		q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
			panic("consumer exploded")
		}))

		q.consumeSafely(Task{TaskID: "abc", Name: "x", RetryCount: 8, RetryMax: 8})

		require.Len(t, storage.failures, 1, "a poison task is eventually given up on")
		require.Equal(t, []string{"abc"}, storage.deleted)
	})
}

// TestConsumeSafely_WorkerSurvives confirms that a worker keeps processing after one Task panics
func TestConsumeSafely_WorkerSurvives(t *testing.T) {

	consumed := make(chan string, 4)

	q := New(
		WithWorkerCount(1),
		withConsumerFuncs(func(task Task) Result {
			if task.Name == "boom" {
				panic("consumer exploded")
			}

			consumed <- task.Name
			return Success()
		}),
	)

	q.Start()
	t.Cleanup(q.Stop)

	require.NoError(t, q.Publish(NewTask("boom", map[string]any{})))
	require.NoError(t, q.Publish(NewTask("after", map[string]any{})))

	select {
	case name := <-consumed:
		require.Equal(t, "after", name, "the worker kept running after the panic")
	case <-time.After(5 * time.Second):
		require.Fail(t, "worker died after the panicking task")
	}
}
