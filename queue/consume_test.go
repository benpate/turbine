package queue

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsume_Success(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
		return Success()
	}))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))

	// A successful task is deleted from storage
	require.Equal(t, []string{"abc"}, storage.deleted)
}

func TestConsume_NoConsumers(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage))

	// With no consumers at all, the task fails at once: logged, and removed from the queue
	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Len(t, storage.failures, 1)
	require.Equal(t, []string{"abc"}, storage.deleted)
	require.Empty(t, storage.saved, "an unrecognized task must never be re-queued")
}

func TestConsume_Ignored_TriesNextConsumer(t *testing.T) {

	storage := &mockStorage{}

	ignoreConsumer := func(Task) Result { return Ignored() }
	acceptConsumer := func(Task) Result { return Success() }

	q := New(WithStorage(storage), withConsumerFuncs(ignoreConsumer, acceptConsumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Equal(t, []string{"abc"}, storage.deleted)
}

func TestConsume_AllIgnored(t *testing.T) {

	storage := &mockStorage{}
	ignoreConsumer := func(Task) Result { return Ignored() }
	q := New(WithStorage(storage), withConsumerFuncs(ignoreConsumer, ignoreConsumer))

	// If every consumer ignores the task, it fails at once: logged, and removed from the queue
	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryMax: 8}))
	require.Len(t, storage.failures, 1)
	require.Equal(t, "x", storage.failures[0].Name)
	require.Equal(t, []string{"abc"}, storage.deleted)
	require.Empty(t, storage.saved, "an unrecognized task must never be re-queued")
}

func TestConsume_Requeue(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithRunImmediatePriority(5), withConsumerFuncs(func(Task) Result {
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
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
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
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
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
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
		return Failure(errors.New("permanent"))
	}))

	// A permanent failure is logged and the task removed
	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Equal(t, 1, len(storage.failures))
	require.Equal(t, []string{"abc"}, storage.deleted)
}

// --- Event handler edge cases ---

func TestOnTaskSuccess_NoStorage(t *testing.T) {
	q := New()
	require.NoError(t, q.onTaskSuccess(consumerFunc(func(Task) Result { return Success() }), Task{TaskID: "abc"}))
}

func TestOnTaskSuccess_StorageError(t *testing.T) {
	storage := &mockStorage{deleteErr: errors.New("db down")}
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
		return Success()
	}))

	// A delete error propagates out of consume
	require.Error(t, q.consume(Task{TaskID: "abc", Name: "x"}))
}

func TestOnTaskError_SaveErrorPropagates(t *testing.T) {

	storage := &mockStorage{saveErr: errors.New("db down")}
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
		return Error(errors.New("temporary"))
	}))

	// A task that cannot be written back for its retry is NOT silently dropped
	require.Error(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 0, RetryMax: 8}))
}

func TestOnTaskFailure_DeleteErrorPropagates(t *testing.T) {

	storage := &mockStorage{deleteErr: errors.New("db down")}
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
		return Failure(errors.New("permanent"))
	}))

	// The failure is logged, but the task could not be removed -- the caller hears about it
	require.Error(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Len(t, storage.failures, 1)
}

func TestOnTaskError_NoStorageUsesBuffer(t *testing.T) {

	q := New(withConsumerFuncs(func(Task) Result {
		return Error(errors.New("temporary"))
	}))

	// With no storage, a retryable error re-queues the task on the in-memory buffer
	require.NoError(t, q.consume(Task{Name: "x", RetryCount: 0, RetryMax: 3}))
	require.Equal(t, 1, len(q.buffer))
}

func TestOnTaskFailure_NoStorage(t *testing.T) {

	q := New(withConsumerFuncs(func(Task) Result {
		return Failure(errors.New("permanent"))
	}))

	// With no storage, a failure is reported and swallowed
	require.NoError(t, q.consume(Task{Name: "x"}))
}

func TestOnTaskFailure_LogError(t *testing.T) {

	storage := &mockStorage{logErr: errors.New("log unavailable")}
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
		return Failure(errors.New("permanent"))
	}))

	require.Error(t, q.consume(Task{TaskID: "abc", Name: "x"}))
}
