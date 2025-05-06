package queue

import (
	"github.com/benpate/derp"
	"github.com/benpate/rosetta/channel"
	"github.com/rs/zerolog/log"
)

// startWorker runs a single worker process, pulling Tasks off
// the buffered channel and running them one at a time.
func (q *Queue) startWorker() {

	// Pull Tasks off of the buffereed channel
	for task := range q.buffer {

		// Execute the Task
		if err := q.consume(task); err != nil {
			derp.Report(err)
		}

		// If the queue has stopped, then exit the worker
		if channel.Closed(q.done) {
			return
		}
	}
}

// consume executes a single Task
func (q *Queue) consume(task Task) error {

	const location = "queue.consume"

	for _, consumeFunc := range q.consumers {

		// Try to run the Task
		result := consumeFunc(task.Name, task.Arguments)

		log.Trace().Str("location", location).Str("name", task.Name).Str("status", result.Status).Msg("Task executed")
		derp.Report(result.Error)

		switch result.Status {

		// If the task was successful, then mark it as complete
		case ResultStatusSuccess:

			log.Trace().Str("location", location).Msg("Task succeeded...")
			if err := q.onTaskSucceeded(task); err != nil {
				return derp.Wrap(err, location, "Error setting task success")
			}

			// UwU :: Return nil == success
			return nil

		// If the Task fails but can be retried, then try to re-queue for another attempt
		case ResultStatusError:

			log.Trace().Str("location", location).Msg("Task error...")

			if err := q.onTaskError(task, result.Error); err != nil {
				return derp.Wrap(err, location, "Error setting task error", result.Error)
			}

			// "successfully" failed, but can be retried
			return nil

		// If the Task fails and should not be retried, then mark it as failed
		case ResultStatusFailure:

			log.Trace().Str("location", location).Msg("Task failure...")

			if err := q.onTaskFailure(task, result.Error); err != nil {
				return derp.Wrap(err, location, "Error setting task failure", result.Error)
			}

			// Task failed successfully
			return nil

		// Unrecognised statuses are the same as "Ignored".
		// If the consumer cannot match this task, then try the next consumer
		// case ResultStatusIgnored:
		default:
			continue
		}
	}

	// No matching consumers found. Return disgrace.
	return derp.InternalError(location, "No consumers available to process task", task)
}
