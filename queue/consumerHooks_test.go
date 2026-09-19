package queue

import (
	"errors"
	"testing"

	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/require"
)

// recordingConsumer is a Consumer that records every lifecycle event it is told about
type recordingConsumer struct {
	result    Result
	events    *[]string
	tasks     *[]Task
	hookError error
	onPublish func(*Task) error
}

// OnPublish implements the Consumer interface
func (consumer recordingConsumer) OnPublish(task *Task) error {
	*consumer.events = append(*consumer.events, "OnPublish")

	if consumer.onPublish != nil {
		return consumer.onPublish(task)
	}

	return nil
}

// Run implements the Consumer interface
func (consumer recordingConsumer) Run(task Task) Result {
	*consumer.events = append(*consumer.events, "Run")
	*consumer.tasks = append(*consumer.tasks, task)
	return consumer.result
}

// OnSuccess implements the Consumer interface
func (consumer recordingConsumer) OnSuccess(Task) error {
	*consumer.events = append(*consumer.events, "OnSuccess")
	return consumer.hookError
}

// OnError implements the Consumer interface
func (consumer recordingConsumer) OnError(Task, error) error {
	*consumer.events = append(*consumer.events, "OnError")
	return consumer.hookError
}

// OnFailure implements the Consumer interface
func (consumer recordingConsumer) OnFailure(Task, error) error {
	*consumer.events = append(*consumer.events, "OnFailure")
	return consumer.hookError
}

// newRecordingConsumer returns a Consumer that always returns `result`, and the slices that
// record what it was told
func newRecordingConsumer(result Result) (recordingConsumer, *[]string, *[]Task) {

	events := make([]string, 0)
	tasks := make([]Task, 0)

	return recordingConsumer{result: result, events: &events, tasks: &tasks}, &events, &tasks
}

// TestConsumerHooks_Success tells the Consumer when its attempt worked
func TestConsumerHooks_Success(t *testing.T) {

	consumer, events, _ := newRecordingConsumer(Success())
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Equal(t, []string{"Run", "OnSuccess"}, *events)
}

// TestConsumerHooks_Requeue reports a requeue as a success, because THIS run succeeded
func TestConsumerHooks_Requeue(t *testing.T) {

	// The trailing OnPublish is not noise: requeueTask re-enters Publish with a fresh identity,
	// so the copy that runs next earns a new pass through the publish-time rules.

	consumer, events, _ := newRecordingConsumer(Requeue(0))
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", Priority: 99}))
	require.Equal(t, []string{"Run", "OnSuccess", "OnPublish"}, *events)
}

// TestConsumerHooks_Error tells the Consumer when its attempt will be tried again
func TestConsumerHooks_Error(t *testing.T) {

	consumer, events, _ := newRecordingConsumer(Error(errors.New("nope")))
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 0, RetryMax: 8}))
	require.Equal(t, []string{"Run", "OnError"}, *events)
}

// TestConsumerHooks_ErrorOnFinalAttempt reports an ABANDONED task as a failure, even though
// the Consumer asked for a retry
func TestConsumerHooks_ErrorOnFinalAttempt(t *testing.T) {

	// The queue has no retries left, so OnError would promise a run that never happens.

	consumer, events, _ := newRecordingConsumer(Error(errors.New("nope")))
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 8, RetryMax: 8}))
	require.Equal(t, []string{"Run", "OnFailure"}, *events)
}

// TestConsumerHooks_Failure tells the Consumer when its task is abandoned outright
func TestConsumerHooks_Failure(t *testing.T) {

	consumer, events, _ := newRecordingConsumer(Failure(errors.New("nope")))
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Equal(t, []string{"Run", "OnFailure"}, *events)
}

// TestConsumerHooks_OnlyTheConsumerThatHandledIt confirms that a Consumer which ignored the task
// hears nothing about how it turned out
func TestConsumerHooks_OnlyTheConsumerThatHandledIt(t *testing.T) {

	ignorer, ignorerEvents, _ := newRecordingConsumer(Ignored())
	handler, handlerEvents, _ := newRecordingConsumer(Success())

	q := New(WithStorage(&mockStorage{}), WithConsumers(ignorer, handler))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Equal(t, []string{"Run"}, *ignorerEvents)
	require.Equal(t, []string{"Run", "OnSuccess"}, *handlerEvents)
}

// TestConsumerHooks_HookErrorDoesNotChangeTheOutcome confirms that a failing hook is reported
// and nothing more -- the queue has already decided what happens to the task
func TestConsumerHooks_HookErrorDoesNotChangeTheOutcome(t *testing.T) {

	// Every hook gets its own case, because each reports its own error -- one passing case no
	// longer speaks for the rest.

	// newFailingConsumer returns a Consumer whose hooks all fail, whatever Run decided
	newFailingConsumer := func(result Result) recordingConsumer {
		events := make([]string, 0)
		tasks := make([]Task, 0)

		return recordingConsumer{
			result:    result,
			events:    &events,
			tasks:     &tasks,
			hookError: errors.New("the hook itself failed"),
		}
	}

	t.Run("OnSuccess", func(t *testing.T) {

		storage := &mockStorage{}
		q := New(WithStorage(storage), WithConsumers(newFailingConsumer(Success())))

		require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))

		// The task is still deleted, as it would be with a silent hook
		require.Equal(t, []string{"abc"}, storage.deleted)
	})

	t.Run("OnError", func(t *testing.T) {

		storage := &mockStorage{}
		q := New(WithStorage(storage), WithConsumers(newFailingConsumer(Error(errors.New("nope")))))

		require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 0, RetryMax: 8}))

		// The retry is still written back, with its failure counted
		require.Len(t, storage.saved, 1)
		require.Equal(t, 1, storage.saved[0].RetryCount)
	})

	t.Run("OnFailure from an abandoned retry", func(t *testing.T) {

		storage := &mockStorage{}
		q := New(WithStorage(storage), WithConsumers(newFailingConsumer(Error(errors.New("nope")))))

		require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 8, RetryMax: 8}))

		// The task is still abandoned to the error log
		require.Len(t, storage.failures, 1)
		require.Equal(t, []string{"abc"}, storage.deleted)
	})

	t.Run("OnFailure from an outright failure", func(t *testing.T) {

		storage := &mockStorage{}
		q := New(WithStorage(storage), WithConsumers(newFailingConsumer(Failure(errors.New("nope")))))

		require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))

		require.Len(t, storage.failures, 1)
		require.Equal(t, []string{"abc"}, storage.deleted)
	})

	t.Run("OnSuccess, inline", func(t *testing.T) {

		q := New(WithConsumers(newFailingConsumer(Success())))

		// An inline task has no storage to check, so the assertion is that the failing hook
		// neither panics nor escapes into the caller
		q.workers.Add(1)
		require.NotPanics(t, func() { q.runInline(Task{Name: "x", Inline: true}) })
	})

	t.Run("OnFailure, inline", func(t *testing.T) {

		q := New(WithConsumers(newFailingConsumer(Error(errors.New("nope")))))

		q.workers.Add(1)
		require.NotPanics(t, func() { q.runInline(Task{Name: "x", Inline: true, RetryMax: 8}) })
	})
}

// TestConsumerHooks_ConsumerSeesTheTask pins what a Consumer is handed
func TestConsumerHooks_ConsumerSeesTheTask(t *testing.T) {

	consumer, _, tasks := newRecordingConsumer(Success())
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	task := Task{
		TaskID:     "abc",
		Name:       "SendEmail",
		Arguments:  mapof.Any{"to": "user@example.com"},
		CreateDate: 1700000000,
		RetryCount: 3,
		RetryMax:   8,
	}

	require.NoError(t, q.consume(task))
	require.Len(t, *tasks, 1)

	got := (*tasks)[0]
	require.Equal(t, "SendEmail", got.Name)
	require.Equal(t, "user@example.com", got.Arguments.GetString("to"))
	require.Equal(t, 3, got.RetryCount)
	require.Equal(t, 8, got.RetryMax)
	require.False(t, got.IsFinalAttempt())
}

// TestTask_IsFinalAttempt pins the retry boundary
func TestTask_IsFinalAttempt(t *testing.T) {

	// These values are measured, not guessed: RetryCount 7 of 8 is still retried, and 8 of 8 is
	// abandoned -- which is why the comparison is `>= RetryMax`.

	require.False(t, Task{RetryCount: 0, RetryMax: 8}.IsFinalAttempt())
	require.False(t, Task{RetryCount: 6, RetryMax: 8}.IsFinalAttempt())
	require.False(t, Task{RetryCount: 7, RetryMax: 8}.IsFinalAttempt())
	require.True(t, Task{RetryCount: 8, RetryMax: 8}.IsFinalAttempt())
	require.True(t, Task{RetryCount: 9, RetryMax: 8}.IsFinalAttempt())

	// RULE: A task with no retries allowed is on its final attempt from the very first run
	require.True(t, Task{RetryCount: 0, RetryMax: 0}.IsFinalAttempt())
}

// TestTask_IsFinalAttempt_MatchesTheQueue proves the predicate agrees with what the queue
// actually does, rather than restating the same arithmetic in two places
func TestTask_IsFinalAttempt_MatchesTheQueue(t *testing.T) {

	for retryCount := range 10 {

		storage := &mockStorage{}
		q := New(WithStorage(storage), withConsumerFuncs(func(Task) Result {
			return Error(errors.New("nope"))
		}))

		task := Task{TaskID: "abc", Name: "x", RetryCount: retryCount, RetryMax: 8}
		require.NoError(t, q.consume(task))

		abandoned := len(storage.failures) > 0
		require.Equal(t, task.IsFinalAttempt(), abandoned, "RetryCount=%d", retryCount)
	}
}

// TestConsumerHooks_Inline reports an inline success, and abandons an inline failure -- an
// Inline task is at-most-once, so it is never retried
func TestConsumerHooks_Inline(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		consumer, events, _ := newRecordingConsumer(Success())
		q := New(WithConsumers(consumer))

		q.workers.Add(1)
		q.runInline(Task{Name: "x", Inline: true})

		require.Equal(t, []string{"Run", "OnSuccess"}, *events)
	})

	t.Run("error is abandoned, not retried", func(t *testing.T) {
		consumer, events, _ := newRecordingConsumer(Error(errors.New("nope")))
		q := New(WithConsumers(consumer))

		q.workers.Add(1)
		q.runInline(Task{Name: "x", Inline: true, RetryMax: 8})

		require.Equal(t, []string{"Run", "OnFailure"}, *events)
	})
}

// hookTaskConsumer records the Task value that each outcome hook is handed
type hookTaskConsumer struct {
	result    Result
	errorTask *Task
	failTask  *Task
}

// OnPublish implements the Consumer interface
func (consumer *hookTaskConsumer) OnPublish(*Task) error { return nil }

// Run implements the Consumer interface
func (consumer *hookTaskConsumer) Run(Task) Result { return consumer.result }

// OnSuccess implements the Consumer interface
func (consumer *hookTaskConsumer) OnSuccess(Task) error { return nil }

// OnError implements the Consumer interface
func (consumer *hookTaskConsumer) OnError(task Task, _ error) error {
	consumer.errorTask = &task
	return nil
}

// OnFailure implements the Consumer interface
func (consumer *hookTaskConsumer) OnFailure(task Task, _ error) error {
	consumer.failTask = &task
	return nil
}

// TestConsumerHooks_HookSeesTheAttemptThatRan pins the Consumer contract: a hook is handed the
// Task as it stood when that attempt began, not the copy prepared for the NEXT one
func TestConsumerHooks_HookSeesTheAttemptThatRan(t *testing.T) {

	consumer := &hookTaskConsumer{result: Error(errors.New("nope"))}
	storage := &mockStorage{}
	q := New(WithStorage(storage), WithConsumers(consumer))

	// RetryCount 7 of 8 is the boundary: this attempt is NOT the last one, but the retry queued
	// behind it will be
	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 7, RetryMax: 8}))

	require.NotNil(t, consumer.errorTask)
	require.Equal(t, 7, consumer.errorTask.RetryCount, "the hook sees the attempt that just failed")
	require.False(t, consumer.errorTask.IsFinalAttempt(), "another attempt is still coming")

	// ...while the Task written back for the next run has this failure counted
	require.Len(t, storage.saved, 1)
	require.Equal(t, 8, storage.saved[0].RetryCount)
	require.True(t, storage.saved[0].IsFinalAttempt(), "the NEXT run is the last one")
}
