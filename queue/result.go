package queue

// ResultStatusSkip represents a task that was not processed because
// the consumer does not recognize it.  The task will be passed to
// another consumer, or error out permanently.
const ResultStatusIgnored = "IGNORED"

// ResultStatusSuccess represents a task that was completed successfully
const ResultStatusSuccess = "SUCCESS"

// ResultStatusError represents a task that was experienced an error, but CAN be retried
const ResultStatusError = "ERROR"

// ResultStatusFailure represents a task that was experienced an error, and CANNOT be retried
const ResultStatusFailure = "FAILURE"

// Result is the return value from a task function
type Result struct {
	Status string
	Error  error
}

// IsSuccessful returns TRUE if the Result is a "SUCCESS"
func (result Result) IsSuccessful() bool {
	return result.Status == ResultStatusSuccess
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
