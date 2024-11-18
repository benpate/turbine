package queue

// Consumer is a function that processes a task from the queue.
type Consumer func(name string, args map[string]any) Result
