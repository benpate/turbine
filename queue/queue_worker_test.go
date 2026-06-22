package queue

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStartWorker_ProcessesBufferedTasks(t *testing.T) {

	var mu sync.Mutex
	consumed := make([]string, 0)

	q := New(WithConsumers(func(name string, _ map[string]any) Result {
		mu.Lock()
		consumed = append(consumed, name)
		mu.Unlock()
		return Success()
	}))

	// Pre-load the buffer with work for the worker to pick up.
	q.buffer <- Task{Name: "a"}
	q.buffer <- Task{Name: "b"}
	q.buffer <- Task{Name: "c"}

	// The worker is a long-lived goroutine; it only returns when done is closed.
	q.workers.Add(1)
	go q.startWorker()

	// Once all three tasks have been consumed, stop the worker.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(consumed) == 3
	}, time.Second, 5*time.Millisecond)

	q.Stop()

	require.ElementsMatch(t, []string{"a", "b", "c"}, consumed)
}

func TestStartWorker_ExitsWhenDoneClosed(t *testing.T) {

	q := New(WithConsumers(func(string, map[string]any) Result {
		return Success()
	}))

	// A worker with an empty buffer is parked waiting for work.
	q.workers.Add(1)
	done := make(chan struct{})
	go func() {
		q.startWorker()
		close(done)
	}()

	// Closing done must wake the idle worker and let it exit (no goroutine leak).
	q.Stop()

	select {
	case <-done:
		// success: the worker returned
	case <-time.After(time.Second):
		t.Fatal("startWorker did not exit after done was closed")
	}
}

func TestStartAndStop_NoStorage(t *testing.T) {

	var count int
	var mu sync.Mutex

	q := New(
		WithWorkerCount(2),
		WithConsumers(func(string, map[string]any) Result {
			mu.Lock()
			count++
			mu.Unlock()
			return Success()
		}),
	)

	q.Start()

	// Publish a handful of tasks (well under bufferSize) and let workers run.
	for i := 0; i < 5; i++ {
		require.NoError(t, q.Publish(NewTask("x", nil)))
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return count == 5
	}, time.Second, 5*time.Millisecond)

	q.Stop()
}
