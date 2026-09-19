package queue

// PreProcessor runs on every Task before it is published, and may modify or reject it.
// Returning an error rejects the Task: it is never queued, and Publish returns that error.
type PreProcessor func(*Task) error
