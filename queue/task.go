package queue

import (
	"time"

	"github.com/benpate/rosetta/mapof"
)

// Task wraps a Task with the metadata required to track its runs and retries.
type Task struct {
	TaskID      string    `bson:"taskId"`              // Unique identfier for this task
	LockID      string    `bson:"lockId,omitempty"`    // Unique identifier for the worker that is currently processing this task
	Name        string    `bson:"name"`                // Name of the task (used to identify the handler function)
	Arguments   mapof.Any `bson:"arguments"`           // Data required to execute this task (marshalled as a map)
	CreateDate  int64     `bson:"createDate"`          // Unix epoch seconds when this task was created
	StartDate   int64     `bson:"startDate"`           // Unix epoch seconds when this task is scheduled to execute
	TimeoutDate int64     `bson:"timeoutDate"`         // Unix epoch seconds when this task will "time out" and can be reclaimed by another process
	Priority    int       `bson:"priority"`            // Priority of the handler, determines the order that tasks are executed in.
	Signature   string    `bson:"signature,omitempty"` // Signature of the task.  If a signature is present, then no other tasks will be allowed with this signature.
	RetryCount  int       `bson:"retryCount"`          // Number of times that this task has already been retried
	RetryMax    int       `bson:"retryMax"`            // Maximum number of times that this task can be retried
	Error       string    `bson:"error,omitempty"`     // Error (if any) from the last execution
	AsyncDelay  int       `bson:"-"`                   // If non-zero, then the `Publish` method will execute in a separate goroutine, and will sleep for this many miliseconds before publishing the Task.
}

// NewTask uses a Task object to create a new Task record
// that can be saved to a Storage provider.
func NewTask(name string, arguments map[string]any, options ...TaskOption) Task {

	now := time.Now().Unix()

	result := Task{
		TaskID:      "",
		LockID:      "",
		Name:        name,
		Arguments:   arguments,
		CreateDate:  now,
		StartDate:   now,
		TimeoutDate: 0,
		Priority:    -1, // If unset, this will be overridden by the Queue object
		RetryMax:    -1, // If unset, this will be overridden by the Queue object
		RetryCount:  0,
		AsyncDelay:  0,
	}

	// Apply functional options
	for _, option := range options {
		option(&result)
	}

	return result
}

// Delay sets the time.Duration before the task is executed
func (task *Task) Delay(delay time.Duration) {
	task.StartDate = time.Now().Add(delay).Unix()
}
