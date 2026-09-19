package queue

import (
	"time"

	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
)

// startWorker runs a single, long-lived worker, pulling Tasks off the buffered
// channel and running them one at a time until Stop closes the done channel.
func (q *Queue) startWorker() {

	// Signal the WaitGroup when this worker exits, so Stop can return.
	defer q.workers.Done()

	for {
		// Block until a Task arrives or the queue is stopped. The done case is
		// what lets an idle worker exit on Stop; without it, a worker parked on
		// <-q.buffer would block forever and leak (buffer is never closed).
		select {

		case <-q.done:
			return

		case task := <-q.buffer:
			q.consumeSafely(task)
		}
	}
}

// consumeSafely executes a single Task and contains any panic it raises.
func (q *Queue) consumeSafely(task Task) {

	const location = "queue.consumeSafely"

	// RULE: A panic in a Consumer would otherwise unwind out of this worker goroutine and take
	// the whole process down, losing every other in-flight Task with it.
	defer func() {

		recovered := recover()

		if recovered == nil {
			return
		}

		err := derp.Internal(location, "Panic in task", task.Name, recovered)
		derp.Report(err)

		// RULE: A panicking Task is routed through the normal error path so that it backs off and
		// is eventually abandoned.  Left alone, its storage record survives, another worker locks
		// it once the lease expires, and it panics again forever.
		if retryErr := q.onTaskError(task, err); retryErr != nil {
			derp.Report(derp.Wrap(retryErr, location, "Unable to record panicking task", task.Name))
		}
	}()

	if err := q.consume(task); err != nil {
		derp.Report(err)
	}
}

// consume executes a single Task by offering it to each consumer in turn,
// stopping at the first one that recognizes (does not ignore) it.
func (q *Queue) consume(task Task) error {

	const location = "queue.consume"

	// Dispatch the task to the consumers until one matches or all have been tried.
	consumer, result, consumed := q.dispatch(task)

	if !consumed {
		return derp.Internal(location, "No consumers available to process task", task)
	}

	switch result.Status {

	// If the task was successful, then mark it as complete
	case ResultStatusSuccess:

		log.Trace().Str("location", location).Msg("Task succeeded.")
		if err := q.onTaskSucceeded(task); err != nil {
			return derp.Wrap(err, location, "Setting task success")
		}

		reportHookError(consumer.OnSuccess(task), "OnSuccess", task.Name)

	// If the task is to be re-queued, then mark it as complete and run it again
	case ResultStatusRequeue:

		log.Trace().Str("location", location).Msg("Task succeeded. Requeuing.")
		if err := q.onTaskSucceeded(task); err != nil {
			return derp.Wrap(err, location, "Setting task success")
		}

		reportHookError(consumer.OnSuccess(task), "OnSuccess", task.Name)

		q.requeueTask(task, result.Delay)

	// If the Task fails but can be retried, then try to re-queue for another attempt
	case ResultStatusError:

		log.Trace().Str("location", location).Msg("Task error...")
		derp.Report(result.Error)

		// RULE: An attempt that fails with no retries left ABANDONS the task, so the Consumer is
		// told the same thing the queue does.  This mirrors the guard inside onTaskError.
		if task.IsFinalAttempt() {
			reportHookError(consumer.OnFailure(task, result.Error), "OnFailure", task.Name)
		} else {
			reportHookError(consumer.OnError(task, result.Error), "OnError", task.Name)
		}

		if err := q.onTaskError(task, result.Error); err != nil {
			return derp.Wrap(err, location, "Setting task error", result.Error)
		}

	// If the Task fails and should not be retried, then mark it as failed
	case ResultStatusFailure:

		log.Trace().Str("location", location).Msg("Task failure...")
		derp.Report(result.Error)

		reportHookError(consumer.OnFailure(task, result.Error), "OnFailure", task.Name)

		if err := q.onTaskFailure(task, result.Error); err != nil {
			return derp.Wrap(err, location, "Setting task failure", result.Error)
		}

	// Unreachable in practice: dispatch already converts an unrecognized status
	// into a Failure.  Kept as a backstop so a new ResultStatus can never be
	// silently treated as success.
	default:
		return derp.Internal(location, "Unrecognized result status", result.Status)
	}

	// Processed "successfully" (though the task may have errored)
	return nil
}

// dispatch offers a Task to each consumer in turn and returns the first one that does not
// ignore it, with its result.  It returns ok=false when no consumer recognized the task.
func (q *Queue) dispatch(task Task) (Consumer, Result, bool) {

	// This is the single home of the consumer-dispatch rule, shared by the
	// pooled workers (consume) and the inline runner (runInline) — only their
	// result POLICIES differ.

	const location = "queue.dispatch"

	for _, consumer := range q.consumers {

		// Try to run the Task.
		// Any recognized statuses mean this consumer handled the task.
		// Return with the result data.
		switch result := consumer.Run(task); result.Status {

		case ResultStatusSuccess,
			ResultStatusRequeue,
			ResultStatusError,
			ResultStatusFailure:

			log.Trace().
				Str("location", location).
				Str("name", task.Name).
				Str("status", result.Status).
				Msg("Task executed")

			return consumer, result, true

		// Ignored status means THIS consumer did not match the task.
		case ResultStatusIgnored:
			continue

		// Unrecognized statuses are bad bad bad. Return an immediate Failure.
		default:
			return consumer, Failure(derp.Internal(location, "Unrecognized result status", result.Status)), true
		}
	}

	// Fall-through means that NO consumer was found that matches the task.
	return nil, Result{}, false
}

// reportHookError reports an error returned by a Consumer's lifecycle hook.  A hook cannot change
// what the queue has already decided about a Task, so its error is reported and nothing else.
func reportHookError(err error, hook string, taskName string) {

	if err == nil {
		return
	}

	derp.Report(derp.Wrap(err, "queue.consume", "Error in Consumer lifecycle hook", hook, taskName))
}

// requeueTask resets a successful task's identity and run counters, then
// publishes a fresh copy to run again after the given delay.
func (q *Queue) requeueTask(task Task, delay time.Duration) {

	const location = "queue.requeueTask"

	// Reset identifying values for this task
	task.TaskID = ""
	task.LockID = ""
	task.Error = ""
	task.StartDate = time.Now().Add(delay).Unix()
	task.TimeoutDate = 0
	task.RetryCount = 0

	// Queue the "new" task
	if err := q.Publish(task); err != nil {
		derp.Report(derp.Wrap(err, location, "Re-publishing task"))
	}
}
