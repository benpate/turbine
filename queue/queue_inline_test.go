package queue

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// NOTE: assertions on inline results happen AFTER q.Stop(), which waits on the
// q.workers WaitGroup that Publish registers every inline goroutine with.  That
// synchronization is itself part of the contract under test.

func TestInline_Runs(t *testing.T) {

	var calls int32
	var gotName atomic.Value

	storage := &mockStorage{}
	q := New(WithStorage(storage), withConsumerFuncs(func(task Task) Result {
		atomic.AddInt32(&calls, 1)
		gotName.Store(task.Name)
		return Success()
	}))

	require.NoError(t, q.Publish(NewTask("nudge", map[string]any{"key": "value"}, WithInline())))
	q.Stop()

	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
	require.Equal(t, "nudge", gotName.Load())

	// Inline tasks never touch storage — not even on success
	require.Empty(t, storage.saved)
	require.Empty(t, storage.deleted)
	require.Empty(t, storage.failures)
}

func TestInline_WorksWithoutStorage(t *testing.T) {

	var calls int32

	// No storage provider: the inline branch must still run (and must not fall
	// into Special Case #1's blocking buffer send).
	q := New(withConsumerFuncs(func(Task) Result {
		atomic.AddInt32(&calls, 1)
		return Success()
	}))

	require.NoError(t, q.Publish(NewTask("nudge", nil, WithInline())))
	q.Stop()

	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestInline_AtMostOnce_ErrorNotRetried(t *testing.T) {

	var calls int32

	storage := &mockStorage{}
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
		atomic.AddInt32(&calls, 1)
		return Error(errors.New("temporary"))
	}))

	require.NoError(t, q.Publish(NewTask("nudge", nil, WithInline())))
	q.Stop()

	// Exactly one attempt: errors are reported and dropped, never requeued
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
	require.Empty(t, storage.saved)
	require.Empty(t, storage.failures)
}

func TestInline_AtMostOnce_FailureNotStored(t *testing.T) {

	var calls int32

	storage := &mockStorage{}
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
		atomic.AddInt32(&calls, 1)
		return Failure(errors.New("permanent"))
	}))

	require.NoError(t, q.Publish(NewTask("nudge", nil, WithInline())))
	q.Stop()

	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
	require.Empty(t, storage.saved)
	require.Empty(t, storage.failures)
	require.Empty(t, storage.deleted)
}

func TestInline_Requeue_Dropped(t *testing.T) {

	var calls int32

	storage := &mockStorage{}
	q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
		atomic.AddInt32(&calls, 1)
		return Requeue(0)
	}))

	require.NoError(t, q.Publish(NewTask("nudge", nil, WithInline())))
	q.Stop()

	// Requeue is a misuse for inline tasks: reported and dropped, not re-run
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
	require.Empty(t, storage.saved)
}

func TestInline_IgnoredTriesNextConsumer(t *testing.T) {

	var accepted int32

	ignoreConsumer := func(Task) Result { return Ignored() }
	acceptConsumer := func(Task) Result {
		atomic.AddInt32(&accepted, 1)
		return Success()
	}

	q := New(withConsumerFuncs(ignoreConsumer, acceptConsumer))

	require.NoError(t, q.Publish(NewTask("nudge", nil, WithInline())))
	q.Stop()

	require.Equal(t, int32(1), atomic.LoadInt32(&accepted))
}

func TestInline_NoConsumers(t *testing.T) {

	// No consumers registered: the runner reports and exits without hanging
	q := New()

	require.NoError(t, q.Publish(NewTask("nudge", nil, WithInline())))
	q.Stop() // must return promptly; a hang here fails via test timeout
}

func TestInline_StopDrains(t *testing.T) {

	var finished int32

	q := New(withConsumerFuncs(func(Task) Result {
		time.Sleep(50 * time.Millisecond)
		atomic.StoreInt32(&finished, 1)
		return Success()
	}))

	require.NoError(t, q.Publish(NewTask("slow-nudge", nil, WithInline())))

	// Stop must wait for the in-flight inline task to complete
	q.Stop()
	require.Equal(t, int32(1), atomic.LoadInt32(&finished))
}

func TestInline_PanicRecovered(t *testing.T) {

	var calls int32

	q := New(withConsumerFuncs(func(Task) Result {
		atomic.AddInt32(&calls, 1)
		panic("boom")
	}))

	// The runner recovers the panic; the process (and this test) survive,
	// and Stop still returns because the WaitGroup is balanced by defers.
	require.NoError(t, q.Publish(NewTask("nudge", nil, WithInline())))
	q.Stop()

	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestInline_SignatureRejected(t *testing.T) {

	var calls int32

	q := New(withConsumerFuncs(func(Task) Result {
		atomic.AddInt32(&calls, 1)
		return Success()
	}))

	// Inline + Signature is contradictory (nothing stored to dedupe against)
	err := q.Publish(NewTask("nudge", nil, WithInline(), WithSignature("sig")))
	require.Error(t, err)

	q.Stop()
	require.Equal(t, int32(0), atomic.LoadInt32(&calls))
}

// TestInline_AfterStop rejects an Inline publish once the queue is stopped: the
// goroutine it would need is registered with the WaitGroup that Stop is already
// waiting on.
func TestInline_AfterStop(t *testing.T) {

	var calls int32

	q := New(WithWorkerCount(0), WithPollStorage(false), withConsumerFuncs(func(Task) Result {
		atomic.AddInt32(&calls, 1)
		return Success()
	}))
	q.Start()
	q.Stop()

	require.Error(t, q.Publish(NewTask("late", nil, WithInline())))
	require.Equal(t, int32(0), atomic.LoadInt32(&calls))
}

func TestTaskOption_WithInline(t *testing.T) {

	task := NewTask("x", nil, WithInline())
	require.True(t, task.Inline)

	// Default is false
	require.False(t, NewTask("x", nil).Inline)
}
