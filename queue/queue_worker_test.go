package queue

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStartWorker_DrainsBuffer(t *testing.T) {

	var mu sync.Mutex
	consumed := make([]string, 0)

	q := New(WithConsumers(func(name string, _ map[string]any) Result {
		mu.Lock()
		consumed = append(consumed, name)
		mu.Unlock()
		return Success()
	}))

	// Pre-load the buffer, then close it so startWorker exits once drained.
	q.buffer <- Task{Name: "a"}
	q.buffer <- Task{Name: "b"}
	q.buffer <- Task{Name: "c"}
	close(q.buffer)

	q.startWorker()

	require.ElementsMatch(t, []string{"a", "b", "c"}, consumed)
}

func TestStartWorker_StopsWhenDoneClosed(t *testing.T) {

	q := New(WithConsumers(func(string, map[string]any) Result {
		return Success()
	}))

	// Signal the worker to stop after its first task
	close(q.done)

	q.buffer <- Task{Name: "a"}
	q.buffer <- Task{Name: "b"}

	// startWorker should process the first task, observe the closed done
	// channel, and return without draining the rest.
	q.startWorker()

	require.Equal(t, 1, len(q.buffer)) // one task remains unprocessed
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
