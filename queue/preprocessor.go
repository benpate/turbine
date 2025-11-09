package queue

// PreProcesor is a custom function that can be added to the Queue.
// This function is executed on tasks BEFORE they are published to the Queue,
// and can be used to modify task properties, or to reject tasks before they are
// executed.  If this function returns an error, then the task is rejected with
// a `queue.Failure` return type, and will not be retried.
//
// PreProcessor is useful for defining centralized rules that apply to all tasks,
// for instance: by setting global prioritization or timing properties.
type PreProcessor func(*Task) error
