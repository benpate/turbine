package queue

import (
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/rosetta/channel"
	"github.com/rs/zerolog/log"
)

type Queue struct {
	storage              Storage       // Storage is the interface to the database
	consumers            []Consumer    // consumers contains all registered Consumer objects
	workerCount          int           // workerCount represents the number of goroutines to use for processing Tasks concurrently. Default process count is 16
	bufferSize           int           // bufferSize determines the number of Tasks to lock in one transaction. Default buffer size is 32
	pollStorage          bool          // pollStorage determines if the queue should poll the database for new tasks. Default is true
	defaultPriority      int           // defaultPriority is the default priority to use when creating new tasks
	runImmediatePriority int           // runImmediatePriority is the maximum priority value that will be tried immediately
	defaultRetryMax      int           // defaultRetryMax is the default number of times to retry a task before giving up
	preProcessor         PreProcessor  // optional pre-processor function that is executed on all tasks before they are published
	buffer               chan Task     // buffer is a channel of tasks that are ready to be processed
	done                 chan struct{} // Done channel is called to stop the queue
	// waitgroup            sync.WaitGroup // Waitgroup to track active workers
}

// New returns a fully initialized Queue object, with all options applied
func New(options ...QueueOption) *Queue {

	// Create the new Queue object
	result := Queue{
		workerCount:          16,
		bufferSize:           32,
		defaultPriority:      16,
		runImmediatePriority: 16,
		defaultRetryMax:      8, // 511 minutes => ~8.5 hours of retries
		pollStorage:          true,
		done:                 make(chan struct{}),
		// waitgroup:            sync.WaitGroup{},    // Initialize the waitgroup
	}

	// Apply options
	for _, option := range options {
		option(&result)
	}

	// Create the task buffer last (to use the correct buffer size)
	result.buffer = make(chan Task, result.bufferSize)

	// Poll the storage container for new Tasks
	go result.start()

	// Start workers to consume tasks
	for i := 0; i < result.workerCount; i++ {
		go result.startWorker()
	}

	// UwU LOL.
	return &result
}

// Start runs the queue and listens for new tasks
func (q *Queue) start() {

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
			derp.Report(err)
			continue
		}

		// If there are no tasks, wait one minute before trying to lock more.
		if len(tasks) == 0 {
			log.Trace().Msg("Turbine Queue: no tasks found.  Waiting 1 minute.")
			time.Sleep(1 * time.Minute)
		}

		// Loop through all tasks that we have to process
		for _, task := range tasks {

			if channel.Closed(q.done) {
				log.Trace().Msg("Turbine Queue: done channel closed. Exiting.")
				return
			}

			q.buffer <- task
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

// PublishTask adds a Task to the Queue
func (q *Queue) Publish(task Task) error {

	const location = "queue.Queue.Publish"

	// Run the pre-processor on the task (if present)
	if q.preProcessor != nil {
		if err := q.preProcessor(&task); err != nil {
			return derp.Wrap(err, location, "Invalid task. Rejected by PreProcessor", task)
		}
	}

	// If the task is to be published asynchronously, then hold it in a goroutine
	if delay := time.Duration(task.AsyncDelay) * time.Millisecond; delay != 0 {
		go func() {
			task.AsyncDelay = 0
			time.Sleep(delay)
			derp.Report(q.Publish(task))
		}()

		// Exit this method (no error reporting possible)
		return nil
	}

	// RULE: Update task.Priority if unset
	if task.Priority == -1 {
		task.Priority = q.defaultPriority
	}

	// RULE: Update task.RetryMax if unset
	if task.RetryMax == -1 {
		task.RetryMax = q.defaultRetryMax
	}

	// Special Case #1: If there is no storage provider,
	// then queue the Task in the memory buffer.  This *may*
	// hold up execution if the buffer is full because there's
	// no storage provider to fall back on.
	if q.storage == nil {
		log.Trace().Msg("Turbine Queue: No storage configured. Task added to channel.")
		q.buffer <- task
		return nil
	}

	// Special Case #2: If the task is marked for immedicate execution, then try to
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
		return derp.Wrap(err, "queue.Push", "Unable to save task to database")
	}

	// Success! (probably)
	return nil
}

func (q *Queue) Schedule(task Task, delay time.Duration) error {

	const location = "queue.Schedule"

	if q.storage == nil {
		return derp.InternalError(location, "Must have a storage provider in order to schedule tasks")
	}

	// Set the task delay time
	task.Delay(delay)

	// Save the Journal to the Storage provider
	if err := q.storage.SaveTask(task); err != nil {
		return derp.Wrap(err, location, "Unable to save task to database")
	}

	return nil
}

func (q *Queue) Delete(signature string) error {
	return q.storage.DeleteTaskBySignature(signature)
}

// Stop closes the queue and stops all workers (after they complete their current task)
func (queue *Queue) Stop() {

	// Send "stop" signal to all workers
	close(queue.done)

	// Wait until all workers have finished their current tasks
	// queue.waitgroup.Wait()
}

// allowImmediate returns TRUE if the Task can be executed immediately
func (queue *Queue) allowImmediate(task *Task) bool {

	// If the task has a signature, then it CANNOT be executed immediately
	if task.Signature != "" {
		return false
	}

	// Otherwise, tasks can execute immediately if their priority is less than or equal to the runImmediatePriority
	return task.Priority <= queue.runImmediatePriority
}
