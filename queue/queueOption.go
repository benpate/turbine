package queue

// Option is a functional option that modifies a Queue object
type Option func(*Queue)

// WithConsumers adds one or more consumers to process tasks from  the Queue
func WithConsumers(consumers ...Consumer) Option {
	return func(q *Queue) {
		q.consumers = append(q.consumers, consumers...)
	}
}

// WithStorage sets the storage and unmarshaller for the Queue
func WithStorage(storage Storage) Option {
	return func(q *Queue) {
		q.storage = storage
	}
}

// WithWorkerCount sets the number of concurrent processes to run
func WithWorkerCount(workerCount int) Option {
	return func(q *Queue) {
		q.workerCount = workerCount
	}
}

// WithBufferSize sets the number of tasks to lock in a single transaction
func WithBufferSize(bufferSize int) Option {
	return func(q *Queue) {
		q.bufferSize = bufferSize
	}
}

// WithPollStorage sets whether the queue should poll the storage for new tasks
func WithPollStorage(pollStorage bool) Option {
	return func(q *Queue) {
		q.pollStorage = pollStorage
	}
}

// WithDefaultPriority sets the default priority for new tasks
func WithDefaultPriority(defaultPriority int) Option {
	return func(q *Queue) {
		q.defaultPriority = defaultPriority
	}
}

// WithRunImmediatePriority sets the maximum priority level for tasks that will run immediately
func WithRunImmediatePriority(runImmediatePriority int) Option {
	return func(q *Queue) {
		q.runImmediatePriority = runImmediatePriority
	}
}

// WithDefaultRetryMax sets the default number of times to retry a task before giving up
func WithDefaultRetryMax(defaultRetryMax int) Option {
	return func(q *Queue) {
		q.defaultRetryMax = defaultRetryMax
	}
}

// WithPreProcessor applies a gloabl taskOption that is
func WithPreProcessor(preProcessor PreProcessor) Option {
	return func(q *Queue) {
		q.preProcessor = preProcessor
	}
}
