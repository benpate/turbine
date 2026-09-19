package queue

import "time"

// TaskOption is a functional option that modifies a Task
type TaskOption func(*Task)

// WithPriority sets the priority of the task
func WithPriority(priority int) TaskOption {
	return func(t *Task) {
		t.Priority = priority
	}
}

// WithInline runs this task at-most-once in its own goroutine, skipping the buffer and the
// storage provider.  Inline tasks are never persisted, retried, or requeued.
func WithInline() TaskOption {
	return func(t *Task) {
		t.Inline = true
	}
}

// WithDelaySeconds delays the task by N seconds from now.  Use WithStartTime for an
// absolute start time.
func WithDelaySeconds(delaySeconds int) TaskOption {
	return func(task *Task) {
		task.Delay(time.Duration(delaySeconds) * time.Second)
	}
}

// WithDelayMinutes delays the task by N minutes from now.  Use WithStartTime for an
// absolute start time.
func WithDelayMinutes(delayMinutes int) TaskOption {
	return func(task *Task) {
		task.Delay(time.Duration(delayMinutes) * time.Minute)
	}
}

// WithDelayHours delays the task by N hours from now.  Use WithStartTime for an
// absolute start time.
func WithDelayHours(delayHours int) TaskOption {
	return func(task *Task) {
		task.Delay(time.Duration(delayHours) * time.Hour)
	}
}

// WithRetryMax sets the maximum number of times that a task can be retried
func WithRetryMax(retryMax int) TaskOption {
	return func(t *Task) {
		t.RetryMax = retryMax
	}
}

// WithSignature sets the task's signature.  Only one task per signature may be active at a
// time, and duplicates are dropped silently.
func WithSignature(signature string) TaskOption {
	return func(t *Task) {
		t.Signature = signature
	}
}

// WithStartTime sets an absolute start time for the task.  Use the WithDelayXXX options for
// a time relative to the current clock.
func WithStartTime(timestamp time.Time) TaskOption {
	return func(t *Task) {
		t.StartDate = timestamp.Unix()
	}
}
