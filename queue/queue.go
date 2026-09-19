package queue

import (
	"sync"
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/rosetta/channel"
	"github.com/rs/zerolog/log"
)

// Queue represents a task queue with support for persistent storage and concurrent processing
type Queue struct {
	storage              Storage        // Storage is the interface to the database
	consumers            []Consumer     // consumers contains all registered Consumer objects
	workerCount          int            // workerCount represents the number of goroutines to use for processing Tasks concurrently. Default process count is 16
	bufferSize           int            // bufferSize determines the number of Tasks to lock in one transaction. Default buffer size is 32
	pollStorage          bool           // pollStorage determines if the queue should poll the database for new tasks. Default is true
	defaultPriority      int            // defaultPriority is the default priority to use when creating new tasks
	runImmediatePriority int            // runImmediatePriority is the maximum priority value that will be tried immediately
	defaultRetryMax      int            // defaultRetryMax is the default number of times to retry a task before giving up
	preProcessor         PreProcessor   // optional pre-processor function that is executed on all tasks before they are published
	buffer               chan Task      // buffer is a channel of tasks that are ready to be processed
	done                 chan struct{}  // done channel is closed to signal all workers to stop
	stopOnce             sync.Once      // stopOnce guarantees the done channel is closed only once, making Stop idempotent
	workers              sync.WaitGroup // workers tracks the running worker goroutines so Stop can wait for them to exit
}

// New returns a fully initialized Queue object, with all options applied
func New(options ...Option) *Queue {

	// Create the new Queue object
	result := Queue{
		workerCount:          16,
		bufferSize:           32,
		defaultPriority:      16,
		runImmediatePriority: 16,
		defaultRetryMax:      8, // backoff sums 2^0..2^7 => 255 minutes (~4.25 hours) of retries
		pollStorage:          true,
		done:                 make(chan struct{}),
	}

	// Apply options
	for _, option := range options {
		option(&result)
	}

	// Create the task buffer last (to use the correct buffer size)
	result.buffer = make(chan Task, result.bufferSize)

	// UwU LOL.
	return &result
}

// Start begins processing tasks in the Queue
func (q *Queue) Start() {

	// Poll the storage container for new Tasks
	go q.start()

	// Start workers to consume tasks. Add to the WaitGroup *before* launching
	// each goroutine so Stop can reliably wait for all of them to exit.
	for i := 0; i < q.workerCount; i++ {
		q.workers.Add(1)
		go q.startWorker()
	}
}

// start runs the queue and listens for new tasks
func (q *Queue) start() {

	const location = "queue.Queue.start"

	// If we don't have a storage object, then we won't poll it for update
	if q.storage == nil {
		return
	}

	// If this service is not configured to poll the database, then return
	if !q.pollStorage {
		return
	}

	log.Trace().Msg("Turbine Queue: polling storage for new tasks")

	// Poll the storage container for new Tasks
	for {

		if channel.Closed(q.done) {
			log.Trace().Msg("Turbine Queue: stopped")
			return
		}

		// Loop through any existing tasks that are locked by this worker
		tasks, err := q.storage.GetTasks()

		if err != nil {
			// Pause before retrying so a failing storage backend doesn't hot-spin this loop.
			derp.Report(derp.Wrap(err, location, "Unable to get tasks from storage"))
			time.Sleep(1 * time.Minute)
			continue
		}

		// If there are no tasks, wait one minute before trying to lock more.
		if len(tasks) == 0 {
			log.Trace().Msg("Turbine Queue: no tasks found.  Waiting 1 minute.")
			time.Sleep(1 * time.Minute)
		}

		// Hand each task to a worker, selecting on q.done as well: a bare send would park this
		// poller forever if Stop fired while the buffer was full.
		for _, task := range tasks {

			select {

			case q.buffer <- task:

			case <-q.done:
				log.Trace().Msg("Turbine Queue: done channel closed. Exiting.")
				return
			}
		}
	}
}

// NewTask pushes a new task to the Queue and swallows any errors that are generated.
func (q *Queue) NewTask(name string, args map[string]any, options ...TaskOption) {
	task := NewTask(name, args, options...)

	if err := q.Publish(task); err != nil {
		derp.Report(err)
	}
}

// Publish adds a Task to the Queue
func (q *Queue) Publish(task Task) error {

	const location = "queue.Queue.Publish"

	// Apply the rules that every published Task passes through
	if err := q.prepareTask(&task); err != nil {
		return derp.Wrap(err, location, "Task rejected", task.Name)
	}

	// Inline tasks bypass the queue machinery entirely: no buffer, no storage.
	// Each one runs at-most-once in its own goroutine, tracked in q.workers so
	// that Stop() drains in-flight inline tasks before returning.
	if task.Inline {

		// RULE: The goroutine below registers with the WaitGroup that Stop is
		// already waiting on, so once q.done is closed there is nothing to run it.
		// (Storage-backed Tasks are still written; the next restart picks them up.)
		if channel.Closed(q.done) {
			return derp.Internal(location, "Queue is stopped. Task not published", task.Name)
		}

		// RULE: Signature dedup requires a stored record to check against; an
		// Inline task is never stored, so the two options cannot combine.
		if task.Signature != "" {
			return derp.Internal(location, "Inline tasks cannot have a Signature", task.Name)
		}

		q.workers.Add(1)
		go q.runInline(task)
		return nil
	}

	// Special Case #1: With no storage provider the Task goes on the memory buffer, which may
	// block the caller when it is full -- there is nowhere else to put it.
	if q.storage == nil {

		// RULE: After Stop, no worker remains to drain the buffer, so fail fast instead of
		// blocking forever.  Storage-backed Tasks skip this guard -- they are still worth
		// writing, because the next restart picks them up.
		if channel.Closed(q.done) {
			return derp.Internal(location, "Queue is stopped. Task not published", task.Name)
		}

		log.Trace().Msg("Turbine Queue: No storage configured. Task added to channel.")

		// Select on q.done as well as the send, so a Stop that lands *during*
		// this call releases the caller too.  Same idiom as start and onTaskError.
		select {

		case q.buffer <- task:
			return nil

		case <-q.done:
			return derp.Internal(location, "Queue is stopped. Task not published", task.Name)
		}
	}

	// Special Case #2: If the task is marked for immediate execution, then try to
	// put it directly into the in-memory buffer.  If the current buffer is full, then
	// write it to disk
	if q.allowImmediate(&task) {
		select {

		case q.buffer <- task:
			log.Trace().Msg("Turbine Queue: Channel available. Task added to channel")
			return nil
		default:
			// If the buffer is full, then fall through and write the Task to the Storage provider
			log.Trace().Msg("Turbine Queue: Channel full. Writing task to storage...")
		}
	}

	// Default Case: Write the Task to the Storage provider
	if err := q.storage.SaveTask(task); err != nil {
		return derp.Wrap(err, location, "Unable to save task to database")
	}

	// Success! (probably)
	return nil
}

// Schedule adds a Task to the Queue to be executed after a delay
func (q *Queue) Schedule(task Task, delay time.Duration) error {

	const location = "queue.Schedule"

	if q.storage == nil {
		return derp.Internal(location, "Must have a storage provider to schedule tasks")
	}

	// RULE: A scheduled Task passes through the same rules as a published one.  Skipping them
	// stored the "unset" sentinels verbatim, and a RetryMax of -1 abandons the Task on its very
	// first failure.
	if err := q.prepareTask(&task); err != nil {
		return derp.Wrap(err, location, "Task rejected", task.Name)
	}

	// Set the task delay time
	task.Delay(delay)

	// Save the Journal to the Storage provider
	if err := q.storage.SaveTask(task); err != nil {
		return derp.Wrap(err, location, "Unable to save task to database")
	}

	return nil
}

// Delete removes a task from the queue by its signature
func (q *Queue) Delete(signature string) error {
	const location = "queue.Queue.Delete"

	// A memory-only queue has no persistent record to delete
	if q.storage == nil {
		return derp.Internal(location, "Must have a storage provider to delete tasks")
	}

	if err := q.storage.DeleteTaskBySignature(signature); err != nil {
		return derp.Wrap(err, location, "Unable to delete task by signature")
	}
	return nil
}

// Stop closes the queue and waits for every worker to finish its current task.
// Stop is idempotent.
func (q *Queue) Stop() {

	// Send "stop" signal to all workers
	q.stopOnce.Do(func() {
		close(q.done)
	})

	// Wait until all workers have finished their current task and exited
	q.workers.Wait()
}

// prepareTask applies every publish-time rule: the PreProcessor, each Consumer's OnPublish,
// and then the Queue's defaults.  Both Publish and Schedule route through it.
func (q *Queue) prepareTask(task *Task) error {

	const location = "queue.Queue.prepareTask"

	// Run the pre-processor on the task (if present)
	if q.preProcessor != nil {
		if err := q.preProcessor(task); err != nil {
			return derp.Wrap(err, location, "Invalid task. Rejected by PreProcessor", task.Name)
		}
	}

	// Offer the Task to every Consumer, each of which may modify it or refuse it outright.
	// They run in registration order, so each one sees what the ones before it did.
	for _, consumer := range q.consumers {
		if err := consumer.OnPublish(task); err != nil {
			return derp.Wrap(err, location, "Invalid task. Rejected by Consumer", task.Name)
		}
	}

	// RULE: The defaults run LAST, so they only fill values that nothing above chose.  They also
	// run after the hooks so that a hook still sees -1 and can tell "unset" from "set to 16".
	if task.Priority == -1 {
		task.Priority = q.defaultPriority
	}

	if task.RetryMax == -1 {
		task.RetryMax = q.defaultRetryMax
	}

	return nil
}

// allowImmediate returns TRUE if the Task can be executed immediately
func (q *Queue) allowImmediate(task *Task) bool {

	// If the task has a signature, then it CANNOT be executed immediately
	if task.Signature != "" {
		return false
	}

	// Otherwise, tasks can execute immediately if their priority is less than or equal to the runImmediatePriority
	return task.Priority <= q.runImmediatePriority
}
