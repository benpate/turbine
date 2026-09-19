package queue

import "time"

// ResultStatusIgnored means this consumer does not recognize the task, which is then offered
// to the next one.  A task NO consumer recognizes fails the run and is left on the queue.
const ResultStatusIgnored = "IGNORED"

// ResultStatusSuccess represents a task that was completed successfully
const ResultStatusSuccess = "SUCCESS"

// ResultStatusRequeue represents a task that completed successfully and should run again at
// the same priority.
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

// NotSuccessful returns TRUE if the Result is neither a "SUCCESS" nor a "REQUEUE"
func (result Result) NotSuccessful() bool {
	return !result.IsSuccessful()
}

// IsNil returns TRUE if the Result is empty (i.e. no status)
func (result Result) IsNil() bool {
	return result.Status == ""
}

// NotNil returns TRUE if the Result is NOT empty (i.e. has a status)
func (result Result) NotNil() bool {
	return result.Status != ""
}

// Ignored returns a Result object that has been "IGNORED"
// This happens when a consumer does not recognize the task name
func Ignored() Result {
	return Result{
		Status: ResultStatusIgnored,
	}
}

// Requeue returns a "REQUEUE" Result: THIS run counts as a success, and a fresh copy of the
// task is published to run again after the delay.
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

// Failure returns a Result object with a status of "FAILURE"
func Failure(err error) Result {
	return Result{
		Status: ResultStatusFailure,
		Error:  err,
	}
}
