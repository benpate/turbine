package queue

import (
	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
)

// runInline executes a single Inline task in its own one-shot goroutine
func (q *Queue) runInline(task Task) {

	const location = "queue.runInline"

	// Inline tasks bypass the buffer and the storage provider, so they run
	// at-most-once: consume's requeue/retry machinery (which writes to storage)
	// must never run for them.  Consumer dispatch is still shared with the
	// pooled workers (see dispatch); only the result POLICY below differs.

	// Signal the WaitGroup on exit.  Publish added this goroutine to q.workers,
	// so Stop waits for in-flight inline tasks before returning.
	defer q.workers.Done()

	// RULE: a panic in a detached goroutine would crash the whole process.
	// Give inline tasks the same isolation a pooled worker enjoys: report it
	// and move on.
	defer func() {
		if r := recover(); r != nil {
			derp.Report(derp.Internal(location, "Panic in inline task", task.Name, r))
		}
	}()

	consumer, result, ok := q.dispatch(task)

	if !ok {
		derp.Report(derp.Internal(location, "No consumers available to process inline task", task.Name))
		return
	}

	switch result.Status {

	// Success: nothing to clean up — the task was never stored.
	case ResultStatusSuccess:
		log.Trace().Str("location", location).Str("name", task.Name).Msg("Inline task succeeded")
		reportHookError(consumer.OnSuccess(task), "OnSuccess", task.Name)

	// At-most-once: a Requeue would re-enter Publish and loop the inline task
	// forever.  Report the misuse and drop the task.
	case ResultStatusRequeue:
		derp.Report(derp.Internal(location, "Inline tasks cannot requeue (at-most-once)", task.Name))

	// At-most-once: errors and failures are reported and dropped.  Retrying
	// would require storage, which Inline tasks must never touch.
	case ResultStatusError, ResultStatusFailure:
		derp.Report(derp.Wrap(result.Error, location, "Inline task did not complete (at-most-once: not retried)", task.Name))

		// RULE: An Inline task is never retried, so a failed one is abandoned here and now --
		// OnError would promise a second attempt that can never happen.
		reportHookError(consumer.OnFailure(task, result.Error), "OnFailure", task.Name)
	}
}
