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

// Consume implements the Consumer interface
func (consumer recordingConsumer) Consume(task Task) Result {
	*consumer.events = append(*consumer.events, "Consume")
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
	require.Equal(t, []string{"Consume", "OnSuccess"}, *events)
}

// TestConsumerHooks_Requeue reports a requeue as a success, because THIS run succeeded.  The
// trailing OnPublish is not noise: requeueTask re-enters Publish with a fresh identity, so the
// copy that runs next is a new publish and earns a new pass through the publish-time rules.
func TestConsumerHooks_Requeue(t *testing.T) {

	consumer, events, _ := newRecordingConsumer(Requeue(0))
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", Priority: 99}))
	require.Equal(t, []string{"Consume", "OnSuccess", "OnPublish"}, *events)
}

// TestConsumerHooks_Error tells the Consumer when its attempt will be tried again
func TestConsumerHooks_Error(t *testing.T) {

	consumer, events, _ := newRecordingConsumer(Error(errors.New("nope")))
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 0, RetryMax: 8}))
	require.Equal(t, []string{"Consume", "OnError"}, *events)
}

// TestConsumerHooks_ErrorOnFinalAttempt reports an ABANDONED task as a failure, even though the
// Consumer asked for a retry.  The queue has no retries left, so OnError would promise a run
// that never happens.
func TestConsumerHooks_ErrorOnFinalAttempt(t *testing.T) {

	consumer, events, _ := newRecordingConsumer(Error(errors.New("nope")))
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 8, RetryMax: 8}))
	require.Equal(t, []string{"Consume", "OnFailure"}, *events)
}

// TestConsumerHooks_Failure tells the Consumer when its task is abandoned outright
func TestConsumerHooks_Failure(t *testing.T) {

	consumer, events, _ := newRecordingConsumer(Failure(errors.New("nope")))
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Equal(t, []string{"Consume", "OnFailure"}, *events)
}

// TestConsumerHooks_OnlyTheConsumerThatHandledIt confirms that a Consumer which ignored the task
// hears nothing about how it turned out
func TestConsumerHooks_OnlyTheConsumerThatHandledIt(t *testing.T) {

	ignorer, ignorerEvents, _ := newRecordingConsumer(Ignored())
	handler, handlerEvents, _ := newRecordingConsumer(Success())

	q := New(WithStorage(&mockStorage{}), WithConsumers(ignorer, handler))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Equal(t, []string{"Consume"}, *ignorerEvents)
	require.Equal(t, []string{"Consume", "OnSuccess"}, *handlerEvents)
}

// TestConsumerHooks_HookErrorDoesNotChangeTheOutcome confirms that a failing hook is reported
// and nothing more -- the queue has already decided what happens to the task
func TestConsumerHooks_HookErrorDoesNotChangeTheOutcome(t *testing.T) {

	events := make([]string, 0)
	tasks := make([]Task, 0)

	consumer := recordingConsumer{
		result:    Success(),
		events:    &events,
		tasks:     &tasks,
		hookError: errors.New("the hook itself failed"),
	}

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))

	// The task is still deleted, exactly as it would be with a silent hook
	require.Equal(t, []string{"abc"}, storage.deleted)
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

// TestTask_IsFinalAttempt pins the retry boundary.  These values are not guesses: a Task with
// RetryCount 7 of 8 is still retried, and one with RetryCount 8 of 8 is abandoned -- which is
// why the comparison is `>= RetryMax` and not `>= RetryMax-1`.
func TestTask_IsFinalAttempt(t *testing.T) {

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

		require.Equal(t, []string{"Consume", "OnSuccess"}, *events)
	})

	t.Run("error is abandoned, not retried", func(t *testing.T) {
		consumer, events, _ := newRecordingConsumer(Error(errors.New("nope")))
		q := New(WithConsumers(consumer))

		q.workers.Add(1)
		q.runInline(Task{Name: "x", Inline: true, RetryMax: 8})

		require.Equal(t, []string{"Consume", "OnFailure"}, *events)
	})
}
