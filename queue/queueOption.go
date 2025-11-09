package queue

// QueueOption is a function that modifies a Queue object
type QueueOption func(*Queue)

func WithConsumers(consumers ...Consumer) QueueOption {
	return func(q *Queue) {
		q.consumers = append(q.consumers, consumers...)
	}
}

// WithStorage sets the storage and unmarshaller for the queue
func WithStorage(storage Storage) QueueOption {
	return func(q *Queue) {
		q.storage = storage
	}
}

// WithWorkerCount sets the number of concurrent processes to run
func WithWorkerCount(workerCount int) QueueOption {
	return func(q *Queue) {
		q.workerCount = workerCount
	}
}

// WithBufferSize sets the number of tasks to lock in a single transaction
func WithBufferSize(bufferSize int) QueueOption {
	return func(q *Queue) {
		q.bufferSize = bufferSize
	}
}

// WithPollStorage sets whether the queue should poll the storage for new tasks
func WithPollStorage(pollStorage bool) QueueOption {
	return func(q *Queue) {
		q.pollStorage = pollStorage
	}
}

// WithDefaultPriority sets the default priority for new tasks
func WithDefaultPriority(defaultPriority int) QueueOption {
	return func(q *Queue) {
		q.defaultPriority = defaultPriority
	}
}

// WithRunImmediatePriority sets the maximum priority level for tasks that will run immediately
func WithRunImmediatePriority(runImmediatePriority int) QueueOption {
	return func(q *Queue) {
		q.runImmediatePriority = runImmediatePriority
	}
}

// WithDefaultRetryMax sets the default number of times to retry a task before giving up
func WithDefaultRetryMax(defaultRetryMax int) QueueOption {
	return func(q *Queue) {
		q.defaultRetryMax = defaultRetryMax
	}
}

// WithPreProcessor applies a gloabl taskOption that is
func WithPreProcessor(preProcessor PreProcessor) QueueOption {
	return func(q *Queue) {
		q.preProcessor = preProcessor
	}
}
