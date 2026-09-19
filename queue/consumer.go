package queue

// Consumer processes Tasks from the queue, and observes what the queue decides about them.
// All five methods are required; a Consumer with nothing to say on an event returns nil.
type Consumer interface {

	// OnPublish is called for every Task on its way onto the queue, before it is stored or run.
	// The pointer lets a Consumer modify the Task; returning an error rejects it.
	OnPublish(task *Task) error

	// Run executes a single attempt of a Task.  A Consumer that does not recognize the Task
	// MUST return Ignored(), so the queue can offer it to the next Consumer.
	Run(task Task) Result

	// OnSuccess is called after an attempt that succeeded, or that requeued itself.
	OnSuccess(task Task) error

	// OnError is called after an attempt that failed and WILL be tried again.
	OnError(task Task, err error) error

	// OnFailure is called when a Task is abandoned and will NOT be tried again.
	OnFailure(task Task, err error) error
}
