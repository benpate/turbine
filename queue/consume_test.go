package queue

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsume_Success(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithConsumers(func(string, map[string]any) Result {
		return Success()
	}))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))

	// A successful task is deleted from storage
	require.Equal(t, []string{"abc"}, storage.deleted)
}

func TestConsume_NoConsumers(t *testing.T) {

	q := New()
	err := q.consume(Task{Name: "x"})
	require.Error(t, err) // "No consumers available to process task"
}

func TestConsume_Ignored_TriesNextConsumer(t *testing.T) {

	storage := &mockStorage{}

	ignoreConsumer := func(string, map[string]any) Result { return Ignored() }
	acceptConsumer := func(string, map[string]any) Result { return Success() }

	q := New(WithStorage(storage), WithConsumers(ignoreConsumer, acceptConsumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Equal(t, []string{"abc"}, storage.deleted)
}

func TestConsume_AllIgnored(t *testing.T) {

	ignoreConsumer := func(string, map[string]any) Result { return Ignored() }
	q := New(WithConsumers(ignoreConsumer, ignoreConsumer))

	// If every consumer ignores the task, consume returns an error
	require.Error(t, q.consume(Task{Name: "x"}))
}

func TestConsume_Requeue(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithRunImmediatePriority(5), WithConsumers(func(string, map[string]any) Result {
		return Requeue(0)
	}))

	// A high-priority task is re-published to storage after success
	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", Priority: 99}))

	// The original was deleted (success) and a fresh copy saved (requeue)
	require.Equal(t, []string{"abc"}, storage.deleted)
	require.Equal(t, 1, len(storage.saved))

	// The re-published task has its identifiers cleared
	require.Equal(t, "", storage.saved[0].TaskID)
	require.Equal(t, 0, storage.saved[0].RetryCount)
}

func TestConsume_Error_Requeues(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithConsumers(func(string, map[string]any) Result {
		return Error(errors.New("temporary"))
	}))

	// A retryable error saves the task back with an incremented retry count
	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 0, RetryMax: 3}))

	require.Equal(t, 1, len(storage.saved))
	require.Equal(t, 1, storage.saved[0].RetryCount)
	require.NotEmpty(t, storage.saved[0].Error)
}

func TestConsume_Error_ExhaustedRetriesFails(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithConsumers(func(string, map[string]any) Result {
		return Error(errors.New("still failing"))
	}))

	// RetryCount >= RetryMax escalates to failure: logged + deleted, not re-saved
	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 3, RetryMax: 3}))

	require.Equal(t, 1, len(storage.failures))
	require.Equal(t, []string{"abc"}, storage.deleted)
	require.Equal(t, 0, len(storage.saved))
}

func TestConsume_Failure(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithConsumers(func(string, map[string]any) Result {
		return Failure(errors.New("permanent"))
	}))

	// A permanent failure is logged and the task removed
	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Equal(t, 1, len(storage.failures))
	require.Equal(t, []string{"abc"}, storage.deleted)
}

// --- Event handler edge cases ---

func TestOnTaskSucceeded_NoStorage(t *testing.T) {
	q := New()
	require.NoError(t, q.onTaskSucceeded(Task{TaskID: "abc"}))
}

func TestOnTaskSucceeded_StorageError(t *testing.T) {
	storage := &mockStorage{deleteErr: errors.New("db down")}
	q := New(WithStorage(storage), WithConsumers(func(string, map[string]any) Result {
		return Success()
	}))

	// A delete error propagates out of consume
	require.Error(t, q.consume(Task{TaskID: "abc", Name: "x"}))
}

func TestOnTaskError_NoStorageUsesBuffer(t *testing.T) {

	q := New(WithConsumers(func(string, map[string]any) Result {
		return Error(errors.New("temporary"))
	}))

	// With no storage, a retryable error re-queues the task on the in-memory buffer
	require.NoError(t, q.consume(Task{Name: "x", RetryCount: 0, RetryMax: 3}))
	require.Equal(t, 1, len(q.buffer))
}

func TestOnTaskFailure_NoStorage(t *testing.T) {

	q := New(WithConsumers(func(string, map[string]any) Result {
		return Failure(errors.New("permanent"))
	}))

	// With no storage, a failure is simply reported and swallowed
	require.NoError(t, q.consume(Task{Name: "x"}))
}

func TestOnTaskFailure_LogError(t *testing.T) {

	storage := &mockStorage{logErr: errors.New("log unavailable")}
	q := New(WithStorage(storage), WithConsumers(func(string, map[string]any) Result {
		return Failure(errors.New("permanent"))
	}))

	require.Error(t, q.consume(Task{TaskID: "abc", Name: "x"}))
}
