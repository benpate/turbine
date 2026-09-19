package queue

// Consumer processes tasks from the queue, and observes what the queue decides about them.
//
// All four methods are REQUIRED.  A Consumer with nothing to say on an event returns nil from
// that method, so that a hook which drifts out of shape fails to compile instead of failing
// silently at run time.
//
// Every method receives the Task as it stood when this run began: `RetryCount` is the number of
// attempts that have already failed, and `Task.IsFinalAttempt()` reports whether a failure now
// will abandon it.
type Consumer interface {

	// Consume executes a single attempt of a Task.  A Consumer that does not recognize the
	// task MUST return Ignored(), so that the queue can offer it to the next Consumer.
	Consume(task Task) Result

	// OnSuccess is called after an attempt that succeeded, or that requeued itself.
	OnSuccess(task Task) error

	// OnError is called after an attempt that failed and WILL be tried again.
	OnError(task Task, err error) error

	// OnFailure is called when a Task is abandoned and will NOT be tried again.
	OnFailure(task Task, err error) error
}
