package queue

import "time"

type TaskOption func(*Task)

// WithPriority sets the priority of the task
func WithPriority(priority int) TaskOption {
	return func(t *Task) {
		t.Priority = priority
	}
}

// WithDelaySeconds sets the number of seconds before the task is executed
// relative to the current clock. This differs from WithStartTime, which
// sets an absolute start time.
func WithDelaySeconds(delaySeconds int) TaskOption {
	return func(task *Task) {
		task.Delay(time.Duration(delaySeconds) * time.Second)
	}
}

// WithDelayMinutes sets the number of minutes before the task is executed
// relative to the current clock. This differs from WithStartTime, which
// sets an absolute start time.
func WithDelayMinutes(delayMinutes int) TaskOption {
	return func(task *Task) {
		task.Delay(time.Duration(delayMinutes) * time.Minute)
	}
}

// WithDelayHours sets the number of hours before the task is executed
// relative to the current clock. This differs from WithStartTime, which
// sets an absolute start time.
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

// WithSignature sets the signature of the task.
// Only one task with a given signature can be active at a time.
// Duplicates are dropped silently.
func WithSignature(signature string) TaskOption {
	return func(t *Task) {
		t.Signature = signature
	}
}

// WithStartTime sets the absolute start time of the task
// This differs from WithDelayXXX options, which set a start time
// relative to the current clock.
func WithStartTime(timestamp time.Time) TaskOption {
	return func(t *Task) {
		t.StartDate = timestamp.Unix()
	}
}
