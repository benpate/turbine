package queue

import (
	"time"

	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
)

// onTaskSuccess marks a task as completed, removes it from the queue, and tells the
// Consumer that handled it.
func (q *Queue) onTaskSuccess(consumer Consumer, task Task) error {

	const location = "queue.onTaskSuccess"

	log.Trace().Str("location", location).Str("name", task.Name).Msg("Task success")

	// Remove the task from the queue.  A memory-only Queue has no stored record to remove,
	// so it reaches the hook below with nothing to do here.
	if q.storage != nil {
		if err := q.storage.DeleteTask(task.TaskID); err != nil {
			return derp.Wrap(err, location, "Unable to remove task from queue")
		}
	}

	// RULE: The Consumer is told only once the queue has recorded the outcome, so a hook that
	// reads storage sees the task already gone.  A delete that fails returns above, un-announced.
	if err := consumer.OnSuccess(task); err != nil {
		derp.Report(derp.Wrap(err, location, "Error in Consumer lifecycle hook", "OnSuccess", task.Name))
	}

	// Silence is golden
	return nil
}

// onTaskError re-queues a failed task for another attempt and tells the Consumer, or hands
// it to onTaskFailure once its retries are spent.
func (q *Queue) onTaskError(consumer Consumer, task Task, err error) error {

	const location = "queue.onTaskError"
	log.Trace().Str("name", task.Name).Str("location", location).Msg("Logging task error")

	// RULE: This guard is the ONLY place the queue decides retry-vs-abandon, and the abandon
	// path reports OnFailure from onTaskFailure -- so the decision and the announcement can
	// never disagree.  `consume` used to re-derive this with Task.IsFinalAttempt().
	if task.RetryCount >= task.RetryMax {
		return q.onTaskFailure(consumer, task, err)
	}

	// Build the NEXT attempt as a separate value.  Mutating `task` here would hand the hook
	// below a Task whose RetryCount already counts this failure, making IsFinalAttempt() answer
	// for the next run instead of the one that just happened.
	retry := task
	retry.LockID = ""
	retry.StartDate = time.Now().Add(backoff(task.RetryCount)).Unix()
	retry.TimeoutDate = 0
	retry.RetryCount = task.RetryCount + 1
	retry.Error = derp.Serialize(err)

	// Re-queue it.  With no storage provider it goes back on the buffer -- selecting on q.done
	// as well, so a worker blocked on a full buffer at shutdown exits cleanly instead of
	// deadlocking against Stop.
	if q.storage == nil {
		log.Trace().Str("location", location).Msg("Storage is nil.  Unable to log error.")
		select {
		case q.buffer <- retry:
		case <-q.done:
		}
	} else if saveErr := q.storage.SaveTask(retry); saveErr != nil {
		return derp.Wrap(saveErr, location, "Unable to re-queue task")
	}

	// RULE: Announce the retry only once it is safely queued.  A nil Consumer is the panic
	// path, which has nobody to tell -- see AGENTS.md.
	if consumer != nil {
		// `err` inside the if is the hook's own failure.  The `err` passed INTO the hook is still
		// the Task's -- Go scopes the new name after the declaration, not during it.
		if err := consumer.OnError(task, err); err != nil {
			derp.Report(derp.Wrap(err, location, "Error in Consumer lifecycle hook", "OnError", task.Name))
		}
	}

	// Silence is golden
	return nil
}

// onTaskFailure marks a task as failed, moves it to the error log, and tells the Consumer
// that handled it.
func (q *Queue) onTaskFailure(consumer Consumer, task Task, err error) error {

	const location = "queue.onTaskFailure"
	log.Trace().Str("location", location).Str("name", task.Name).Msg("Logging task failure...")

	// RULE: This is the ONE place a task's own error is reported: a Failure, or an Error whose
	// retries have run out.  Retries themselves are never reported.
	derp.Report(err)

	// Record the error on a separate value, so the hook below still sees the attempt as it ran
	failed := task
	failed.Error = derp.Message(err)

	// Move the task to the error log and off the queue.  A memory-only Queue has no error log.
	if q.storage == nil {

		log.Trace().Str("location", location).Msg("Storage is nil.  Unable to log failure.")

	} else {

		// Not named `err`: that would shadow the failure the hook below reports
		if logErr := q.storage.LogFailure(failed); logErr != nil {
			return derp.Wrap(logErr, location, "Unable to add task to error log")
		}

		if deleteErr := q.storage.DeleteTask(failed.TaskID); deleteErr != nil {
			return derp.Wrap(deleteErr, location, "Unable to remove task from queue")
		}
	}

	// RULE: The Consumer is told only once the queue has recorded the outcome, so a hook that
	// reads storage sees the task already gone.
	if consumer != nil {
		// The inner `err` is the hook's own failure, not the Task's -- see onTaskError.
		if err := consumer.OnFailure(task, err); err != nil {
			derp.Report(derp.Wrap(err, location, "Error in Consumer lifecycle hook", "OnFailure", task.Name))
		}
	}

	// Succeeded in logging the failure, even if the Task itself failed.
	return nil
}
