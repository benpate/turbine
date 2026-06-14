package queue

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// pollStorage is a Storage whose GetTasks always returns a fixed batch, used to
// drive the queue's polling loop in start().
type pollStorage struct {
	mockStorage
	batch []Task
}

func (p *pollStorage) GetTasks() ([]Task, error) {
	return p.batch, nil
}

func TestStart_NoStorage(t *testing.T) {
	// With no storage, start() returns immediately.
	q := New()
	q.start() // must not block or panic
}

func TestStart_PollingDisabled(t *testing.T) {
	// With polling disabled, start() returns immediately.
	q := New(WithStorage(&mockStorage{}), WithPollStorage(false))
	q.start()
}

func TestStart_PollsStorageThenStops(t *testing.T) {

	storage := &pollStorage{
		batch: []Task{{Name: "a"}, {Name: "b"}},
	}

	q := New(WithStorage(storage))

	// Drain the buffer in the background so start()'s sends never block.
	received := make(chan Task, 100)
	go func() {
		for task := range q.buffer {
			received <- task
		}
	}()

	finished := make(chan struct{})
	go func() {
		q.start()
		close(finished)
	}()

	// Confirm the poller is feeding tasks into the buffer
	for i := 0; i < 3; i++ {
		select {
		case <-received:
		case <-time.After(time.Second):
			t.Fatal("expected polled tasks")
		}
	}

	// Stop the queue; start() should observe the closed done channel and exit.
	q.Stop()

	select {
	case <-finished:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("start() did not stop after Stop()")
	}
}

func TestQueue_NewTask_SwallowsError(t *testing.T) {

	// NewTask reports (but does not return) errors from Publish. A rejecting
	// pre-processor makes Publish fail; NewTask must not panic.
	q := New(WithPreProcessor(func(*Task) error {
		return errors.New("rejected")
	}))

	require.NotPanics(t, func() {
		q.NewTask("x", nil)
	})
	require.Equal(t, 0, len(q.buffer))
}
