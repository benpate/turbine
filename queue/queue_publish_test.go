package queue

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPublish_NilStorageBuffer(t *testing.T) {

	// With no storage and an open buffer, Publish writes to the channel.
	// We stay well under bufferSize (32) to avoid blocking.
	q := New()

	require.NoError(t, q.Publish(NewTask("x", nil)))
	require.Equal(t, 1, len(q.buffer))

	// Defaults are applied to the queued task
	task := <-q.buffer
	require.Equal(t, 16, task.Priority) // defaultPriority
	require.Equal(t, 8, task.RetryMax)  // defaultRetryMax
}

func TestPublish_DefaultPathToStorage(t *testing.T) {

	// A high-priority task (above runImmediatePriority) is written to storage,
	// not the in-memory buffer.
	storage := &mockStorage{}
	q := New(WithStorage(storage), WithRunImmediatePriority(5))

	require.NoError(t, q.Publish(NewTask("x", nil, WithPriority(99))))
	require.Equal(t, 1, len(storage.saved))
	require.Equal(t, 0, len(q.buffer))
}

func TestPublish_ImmediateToBuffer(t *testing.T) {

	// A low-priority task with storage available still goes to the buffer first
	// when there is room.
	storage := &mockStorage{}
	q := New(WithStorage(storage))

	require.NoError(t, q.Publish(NewTask("x", nil, WithPriority(1))))
	require.Equal(t, 1, len(q.buffer))
	require.Equal(t, 0, len(storage.saved))
}

func TestPublish_ImmediateBufferFullFallsToStorage(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithBufferSize(1))

	// Fill the single buffer slot
	require.NoError(t, q.Publish(NewTask("x", nil, WithPriority(1))))
	require.Equal(t, 1, len(q.buffer))

	// The next immediate task can't fit, so it falls through to storage
	require.NoError(t, q.Publish(NewTask("y", nil, WithPriority(1))))
	require.Equal(t, 1, len(storage.saved))
}

func TestPublish_StorageError(t *testing.T) {

	storage := &mockStorage{saveErr: errors.New("db down")}
	q := New(WithStorage(storage), WithRunImmediatePriority(5))

	err := q.Publish(NewTask("x", nil, WithPriority(99)))
	require.Error(t, err)
}

func TestPublish_PreProcessorReject(t *testing.T) {

	q := New(WithPreProcessor(func(*Task) error {
		return errors.New("rejected")
	}))

	err := q.Publish(NewTask("x", nil))
	require.Error(t, err)
	require.Equal(t, 0, len(q.buffer))
}

func TestPublish_PreProcessorModify(t *testing.T) {

	q := New(WithPreProcessor(func(task *Task) error {
		task.Name = "modified"
		return nil
	}))

	require.NoError(t, q.Publish(NewTask("original", nil)))
	task := <-q.buffer
	require.Equal(t, "modified", task.Name)
}

func TestPublish_AsyncDelay(t *testing.T) {

	q := New()

	// An async task returns immediately and is published from a goroutine.
	require.NoError(t, q.Publish(NewTask("x", nil, WithAsyncDelay(20))))
	require.Equal(t, 0, len(q.buffer)) // not published yet

	// Wait for the async goroutine to publish
	require.Eventually(t, func() bool {
		return len(q.buffer) == 1
	}, time.Second, 5*time.Millisecond)
}

func TestQueue_NewTask(t *testing.T) {

	// NewTask is the error-swallowing convenience wrapper around Publish
	q := New()
	q.NewTask("x", map[string]any{"k": "v"})
	require.Equal(t, 1, len(q.buffer))
}

func TestSchedule_NoStorage(t *testing.T) {
	q := New()
	err := q.Schedule(NewTask("x", nil), time.Minute)
	require.Error(t, err)
}

func TestSchedule_WithStorage(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage))

	require.NoError(t, q.Schedule(NewTask("x", nil), time.Hour))
	require.Equal(t, 1, len(storage.saved))

	// The scheduled start time is in the future
	require.Greater(t, storage.saved[0].StartDate, time.Now().Unix())
}

func TestSchedule_StorageError(t *testing.T) {
	storage := &mockStorage{saveErr: errors.New("db down")}
	q := New(WithStorage(storage))
	require.Error(t, q.Schedule(NewTask("x", nil), time.Minute))
}

func TestDelete(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage))

	require.NoError(t, q.Delete("my-signature"))
	require.Equal(t, []string{"my-signature"}, storage.deletedBySig)
}

func TestStop_ClosesDone(t *testing.T) {

	q := New()
	q.Stop()

	// After Stop, the done channel is closed
	select {
	case <-q.done:
		// expected: closed channel returns immediately
	default:
		t.Fatal("expected done channel to be closed")
	}
}

func TestAllowImmediate(t *testing.T) {

	q := New(WithRunImmediatePriority(10))

	// Priority at or below the threshold is allowed
	require.True(t, q.allowImmediate(&Task{Priority: 5}))
	require.True(t, q.allowImmediate(&Task{Priority: 10}))

	// Priority above the threshold is not allowed
	require.False(t, q.allowImmediate(&Task{Priority: 11}))

	// A task with a signature is never immediate
	require.False(t, q.allowImmediate(&Task{Priority: 1, Signature: "sig"}))
}
