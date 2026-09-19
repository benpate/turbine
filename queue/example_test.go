package queue

import "fmt"

// ExampleNewTask shows how to construct a Task with functional options.
// Priority and RetryMax default to -1 ("unset") and are replaced by the
// queue's defaults when the task is published.
func ExampleNewTask() {

	task := NewTask("SendEmail", map[string]any{"to": "user@example.com"}, WithPriority(8))

	fmt.Println("name:", task.Name)
	fmt.Println("priority:", task.Priority)
	fmt.Println("to:", task.Arguments["to"])

	// Output:
	// name: SendEmail
	// priority: 8
	// to: user@example.com
}

// printConsumer is a Consumer that prints the name of every task it receives.  Every Consumer
// implements all four methods; one with nothing to say on an event returns nil.
type printConsumer struct{}

// OnPublish implements the Consumer interface
func (consumer printConsumer) OnPublish(*Task) error { return nil }

// Run implements the Consumer interface
func (consumer printConsumer) Run(task Task) Result {
	fmt.Println("received task:", task.Name)
	return Success()
}

// OnSuccess implements the Consumer interface
func (consumer printConsumer) OnSuccess(Task) error { return nil }

// OnError implements the Consumer interface
func (consumer printConsumer) OnError(Task, error) error { return nil }

// OnFailure implements the Consumer interface
func (consumer printConsumer) OnFailure(Task, error) error { return nil }

// ExampleQueue_Publish runs a task end-to-end on a memory-only queue.  The
// task is marked WithInline, so Publish executes it at-most-once in its own
// goroutine — no storage provider and no Start() worker pool are needed —
// and Stop() waits for it to finish.
func ExampleQueue_Publish() {

	q := New(WithConsumers(printConsumer{}))

	task := NewTask("SendEmail", map[string]any{}, WithInline())

	if err := q.Publish(task); err != nil {
		fmt.Println("error:", err)
	}

	// Stop drains in-flight inline tasks before returning
	q.Stop()
	fmt.Println("queue stopped")

	// Output:
	// received task: SendEmail
	// queue stopped
}
