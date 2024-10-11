package queue

import (
	"github.com/benpate/derp"
	"github.com/benpate/rosetta/channel"
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

	const location = "queue.processOne"

	for _, consumeFunc := range q.consumers {

		// Try to run the Task
		matchFound, runError := consumeFunc(task.Name, task.Arguments)

		// If this consumer cannot match this task, then try the next consumer
		if !matchFound {
			continue
		}

		// If there was an error running the consumer, then re-queue (or fail) the Task
		if runError != nil {

			// If the Task fails, then try to re-queue or handle the error
			if writeError := q.onTaskError(task, runError); writeError != nil {
				return derp.Wrap(writeError, location, "Error setting task error", runError)
			}

			// Report the error but do not return it because we have re-queued the task to try again
			// derp.Report(derp.Wrap(runError, location, "Error executing task"))
			return nil
		}

		// Otherwise, the Task was successful.  Remove it from the Storage provider
		if runError := q.onTaskSucceeded(task); runError != nil {
			return derp.Wrap(runError, location, "Error setting task success")
		}

		// UwU :: Return nil == success
		return nil
	}

	// No matching consumers found. Return disgrace.
	return derp.NewInternalError(location, "No consumers available to process task", task)
}
