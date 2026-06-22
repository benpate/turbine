package queue

import (
	"time"

	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
)

// startWorker runs a single, long-lived worker process, pulling Tasks off
// the buffered channel and running them one at a time. It blocks waiting for
// new work for the entire lifecycle of the Queue, and only exits when Stop
// closes the done channel.
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
			if err := q.consume(task); err != nil {
				derp.Report(err)
			}
		}
	}
}

// consume executes a single Task by offering it to each consumer in turn,
// stopping at the first one that recognizes (does not ignore) it.
func (q *Queue) consume(task Task) error {

	const location = "queue.consume"

	for _, consumeFunc := range q.consumers {

		// Try to run the Task
		result := consumeFunc(task.Name, task.Arguments)

		log.Trace().Str("location", location).Str("name", task.Name).Str("status", result.Status).Msg("Task executed")
		derp.Report(result.Error)

		// Apply the result. If this consumer ignored the task, try the next one.
		handled, err := q.applyResult(task, result)

		if !handled {
			continue
		}

		if err != nil {
			return derp.Wrap(err, location, "Applying result for task", task)
		}

		return nil
	}

	// No matching consumers found. Return disgrace.
	return derp.Internal(location, "No consumers available to process task", task)
}

// applyResult records the outcome of a single consumer run. It returns
// handled=false when the consumer ignored the task (so the caller tries the
// next consumer), and handled=true once a consumer has owned the result.
func (q *Queue) applyResult(task Task, result Result) (bool, error) {

	const location = "queue.applyResult"

	switch result.Status {

	// If the task was successful, then mark it as complete
	case ResultStatusSuccess:

		log.Trace().Str("location", location).Msg("Task succeeded.")
		if err := q.onTaskSucceeded(task); err != nil {
			return true, derp.Wrap(err, location, "Setting task success")
		}

		return true, nil

	// If the task is to be re-queued, then mark it as complete and run it again
	case ResultStatusRequeue:

		log.Trace().Str("location", location).Msg("Task succeeded. Requeuing.")
		if err := q.onTaskSucceeded(task); err != nil {
			return true, derp.Wrap(err, location, "Setting task success")
		}

		q.requeueTask(task, result.Delay)
		return true, nil

	// If the Task fails but can be retried, then try to re-queue for another attempt
	case ResultStatusError:

		log.Trace().Str("location", location).Msg("Task error...")

		if err := q.onTaskError(task, result.Error); err != nil {
			return true, derp.Wrap(err, location, "Setting task error", result.Error)
		}

		// "successfully" failed, but can be retried
		return true, nil

	// If the Task fails and should not be retried, then mark it as failed
	case ResultStatusFailure:

		log.Trace().Str("location", location).Msg("Task failure...")

		if err := q.onTaskFailure(task, result.Error); err != nil {
			return true, derp.Wrap(err, location, "Setting task failure", result.Error)
		}

		// Task failed successfully
		return true, nil

	// Unrecognised statuses are the same as "Ignored": this consumer did not
	// handle the task, so the caller should try the next consumer.
	default:
		return false, nil
	}
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
