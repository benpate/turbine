# ⚙️ Turbine

<img src="https://github.com/benpate/turbine/raw/main/meta/banner.webp" style="width:100%; display:block; margin-bottom:20px;"  alt="Oil painting titled: Dutch landscape with windmills, by Gerard Delfgaauw (1882-1947)">


[![Go Reference](https://pkg.go.dev/badge/github.com/benpate/turbine.svg)](https://pkg.go.dev/github.com/benpate/turbine)
[![Version](https://img.shields.io/github/v/release/benpate/turbine?include_prereleases&style=flat-square&color=brightgreen)](https://github.com/benpate/turbine/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/benpate/turbine/go.yml?style=flat-square)](https://github.com/benpate/turbine/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/benpate/turbine?style=flat-square)](https://goreportcard.com/report/github.com/benpate/turbine)
[![Codecov](https://img.shields.io/codecov/c/github/benpate/turbine.svg?style=flat-square)](https://codecov.io/gh/benpate/turbine)

## Fast, Distributed Message Queue for Go and MongoDB

There are many, many message queue tools. Perhaps you should use one of those instead. But Turbine fills a unique niche in that it:

1. is a distributed queue that can be shared between several producer and consumer servers simultaneously
2. supports swappable storage providers, with the first provider being MongoDB.
3. supports fast, in-memory queues using Golang channels
4. can retry failed jobs (with exponential backoff)
5. can schedule jobs in the future

## Run the Queue

```go
// Create and start a queue
q := queue.New(
    queue.WithConsumers(myConsumer),  // one or more "Consumer" implementations (below)
    queue.WithStorage(provider),      // optional storage adapter persists tasks
    queue.WithWorkerCount(16),        // other configs: worker count, buffer size, retries, etc.
)

q.Start() // launch the worker goroutines
defer q.Stop()
```

## Pushing Tasks to the Queue

```go
// Create a Task with any parameters
task := queue.NewTask(
    "TaskName", // Task Name
    map[string]any{ // Parameters
        "foo": "bar",
        "baz": 42,
    },
)

// Publish the task to the Queue (this will happen quickly)
if err := q.Publish(task); err != nil {
    // only errors related to queuing the task
}
```

## Consuming Tasks from the Queue

When the turbine queue receives a task, it tries to execute it using one or more `Consumer`
implementations:

```go
type Consumer interface {
    Consume(task queue.Task) queue.Result
    OnSuccess(task queue.Task) error
    OnError(task queue.Task, err error) error
    OnFailure(task queue.Task, err error) error
}
```

`Consume` does the work.  The other three are how the queue tells a consumer what it decided,
and a consumer with nothing to say on an event returns `nil`.  All four are required on purpose:
an optional hook whose name or signature drifted would stop being called with no compile error
and no log line.

Every method receives the `Task` as it stood when this run began, so `RetryCount` is the number
of attempts that have already failed.  `task.IsFinalAttempt()` reports whether a failure now
will abandon the task — though a consumer that only cares about giving up should use
`OnFailure`, which the queue calls for it.

It is the consumer's job to identify the task by name, and decide if it can
execute it or not.  You can configure any number of consumers, so if
a task is not recognized (the consumer returns `queue.Ignored()`), it is passed
to the next consumer until a match is found.  Only the consumer that handled the task
hears about how it turned out.

If the consumer DOES recognize the Task, then it executes the job and returns one
of the `queue.Result` constructors:

- `queue.Success()` — the task completed successfully
- `queue.Error(err)` — the task failed but CAN be retried
- `queue.Failure(err)` — the task failed and should NOT be retried
- `queue.Requeue(delay)` — the task succeeded and should run again after `delay`
- `queue.Ignored()` — this consumer does not handle this task

When a consumer returns `queue.Error`, the task is re-queued according to Turbine's exponential backoff logic, and will be re-run at some point in the future.  `OnError` is called for each of those retries; when the retries run out, the task is abandoned and `OnFailure` is called instead.

## Scheduling and Deleting Tasks

Both of these methods require a storage provider; on a memory-only queue they return an error.

```go
// Schedule a task to run after a delay (instead of immediately)
if err := q.Schedule(task, 30*time.Minute); err != nil {
    // only errors related to storing the task
}
```

Use a `Signature` to guarantee that only one copy of a task is active at a time. A queued task can then be removed by that same signature:

```go
task := queue.NewTask("TaskName", args, queue.WithSignature("daily-report"))

// Remove a queued task by its signature
if err := q.Delete("daily-report"); err != nil {
    // only errors related to deleting the task
}
```

## Inline Tasks

Use `WithInline` for short-lived, volatile, process-local work (such as realtime
notifications) where a stored or cross-process run would be meaningless.  An inline
task runs at-most-once in its own goroutine the moment it is published — it is never
buffered, never written to storage, and never retried or requeued.  A failed inline
task is reported and dropped.  Inline tasks cannot have a `Signature` (there is no
stored record to de-duplicate against).

```go
task := queue.NewTask("RealtimeNudge", args, queue.WithInline())

if err := q.Publish(task); err != nil {
    // only errors related to validating the task
}
```

## Mongo Storage Provider

Turbine is built to support pluggable storage providers, so that any datastore can be used to manage queued tasks.

Currently, there is a single storage provider for MongoDB, which safely queues and dequeues tasks for any number of distributed queue workers. However, [storage providers implement a simple interface](https://pkg.go.dev/github.com/benpate/turbine@v0.1.0/queue#Storage), so it is simple to create a new storage provider for any back end that you want to use.

**IMPORTANT**: If you do not use a storage provider, the Turbine queue will still work, but will only work in memory. This means that items cannot be queued for future dates, and will not have a retry delay.

To initialize the storage provider, use the following code:

```go
import (
    "github.com/benpate/turbine/queue"
    "github.com/benpate/turbine/queue_mongo"
)

// Connect to MongoDB and select a database
client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://..."))
database := client.Database("myDatabase")

// Pass the database into the storage provider, along with the number of
// tasks to lock per batch and the lock timeout (in minutes)
provider := queue_mongo.New(database, 32, 5)

// Initialize the queue with the storage provider
q := queue.New(queue.WithStorage(provider))
```

## Image Credit

The banner is *Dutch landscape with windmills* by Gerard Delfgaauw (1882–1947). The work is in the public domain.

## Pull Requests Welcome

I'm trying to make Turbine the best it can be, and your help is greatly appreciated. If you find a bug or have an idea for a new feature, please open an issue or submit a pull request. We're all in this together! ⚙️
