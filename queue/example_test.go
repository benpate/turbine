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

// ExampleQueue_Publish runs a task end-to-end on a memory-only queue.  The
// task is marked WithInline, so Publish executes it at-most-once in its own
// goroutine — no storage provider and no Start() worker pool are needed —
// and Stop() waits for it to finish.
func ExampleQueue_Publish() {

	q := New(WithConsumers(func(name string, _ map[string]any) Result {
		fmt.Println("received task:", name)
		return Success()
	}))

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
