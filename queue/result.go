package queue

import "time"

// ResultStatusIgnored represents a task that was not processed because
// the consumer does not recognize it.  The task will be passed to
// another consumer, or error out permanently.
const ResultStatusIgnored = "IGNORED"

// ResultStatusSuccess represents a task that was completed successfully
const ResultStatusSuccess = "SUCCESS"

// ResultStatusRequeue represents a task that has completed successfully
// and should be re-executed at its same priority level.  This is useful
// for long series of tasks that need to execute over multiple records.
const ResultStatusRequeue = "REQUEUE"

// ResultStatusError represents a task that was experienced an error,
// but CAN be retried
const ResultStatusError = "ERROR"

// ResultStatusFailure represents a task that was experienced an error,
// and CANNOT be retried
const ResultStatusFailure = "FAILURE"

// Result is the return value from a task function
type Result struct {
	Status string
	Error  error
	Delay  time.Duration
}

// IsSuccessful returns TRUE if the Result is a "SUCCESS" or "REQUEUE"
func (result Result) IsSuccessful() bool {
	return (result.Status == ResultStatusSuccess) || (result.Status == ResultStatusRequeue)
}

// NotSuccessful returns TRUE if the Result is NOT a "SUCCESS"
func (result Result) NotSuccessful() bool {
	return !result.IsSuccessful()
}

// Ignored returns a Result object that has been "IGNORED"
// This happens when a consumer does not recognize the task name
func Ignored() Result {
	return Result{
		Status: ResultStatusIgnored,
	}
}

// Requeue returns a Result object that will be "REQUEUED"
// which marks THIS task as successful, and also pushes a
// copy of this task back onto the queue to run again.
func Requeue(delay time.Duration) Result {
	return Result{
		Status: ResultStatusRequeue,
		Delay:  delay,
	}
}

// Success returns a Result object with a status of "SUCCESS"
func Success() Result {
	return Result{
		Status: ResultStatusSuccess,
	}
}

// Error returns a Result object with a status of "ERROR"
func Error(err error) Result {
	return Result{
		Status: ResultStatusError,
		Error:  err,
	}
}

// Failure returns a Result object with a status of "HALT"
func Failure(err error) Result {
	return Result{
		Status: ResultStatusFailure,
		Error:  err,
	}
}
