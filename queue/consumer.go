package queue

// Consumer processes tasks from the queue, and observes what the queue decides about them.
//
// All five methods are REQUIRED.  A Consumer with nothing to say on an event returns nil from
// that method, so that a hook which drifts out of shape fails to compile instead of failing
// silently at run time.
//
// Run and the three outcome hooks receive the Task as it stood when that run began:
// `RetryCount` is the number of attempts that have already failed, and `Task.IsFinalAttempt()`
// reports whether a failure now will abandon it.
type Consumer interface {

	// OnPublish is called for every Task on its way onto the queue, before it is stored or run,
	// and before the Queue fills in its defaults -- so an unset Priority or RetryMax is still -1.
	// The Task is passed by POINTER because a Consumer may modify it here; returning an error
	// rejects the Task, and Publish reports that error to whoever tried to publish it.
	//
	// Every registered Consumer sees every Task, in registration order, each one seeing what the
	// ones before it did.  This is the per-Consumer replacement for WithPreProcessor.
	OnPublish(task *Task) error

	// Run executes a single attempt of a Task.  A Consumer that does not recognize the
	// task MUST return Ignored(), so that the queue can offer it to the next Consumer.
	Run(task Task) Result

	// OnSuccess is called after an attempt that succeeded, or that requeued itself.
	OnSuccess(task Task) error

	// OnError is called after an attempt that failed and WILL be tried again.
	OnError(task Task, err error) error

	// OnFailure is called when a Task is abandoned and will NOT be tried again.
	OnFailure(task Task, err error) error
}
