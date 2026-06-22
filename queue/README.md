# queue

The core of [Turbine](../README.md): a distributed task queue with swappable storage back ends and concurrent, in-memory workers. Storage providers (such as [queue_mongo](../queue_mongo/)) implement the `Storage` interface in this package.

## What matters here

- **Workers are permanent goroutines; `Stop()` blocks until they exit.** `Start()` launches `workerCount` workers that each block on a `select` over the task buffer and the `done` channel for the entire lifecycle of the queue. They are *not* meant to be torn down between tasks — only `Stop()` (which closes `done`) ends them. `Stop()` waits on a `sync.WaitGroup`, so it returns only after every in-flight task finishes; the worker must `select` on `done` (not `range` the buffer) or an idle worker would block forever and leak.
- **No storage provider = in-memory only.** With no `Storage`, tasks live solely in the buffered channel: they cannot be scheduled for the future, and failed tasks are re-queued with *no* backoff delay. Future scheduling and retry delays require a persistent provider.
- **Consumers return a `Result`, not `(bool, error)`.** A consumer signals outcome via the `Result` constructors (`Success`, `Error`, `Failure`, `Requeue`, `Ignored`). Returning `Ignored()` (or any unrecognized status) passes the task to the *next* registered consumer — this is how task dispatch works, so a consumer must ignore names it doesn't own.
- **`Error` retries; `Failure` does not.** `queue.Error(err)` re-queues with exponential backoff (`backoff()` waits `2^retryCount` minutes) until `RetryCount` reaches the task's `RetryMax`, after which it is treated as a failure; `queue.Failure(err)` moves the task straight to the error log immediately. Choosing the wrong one either drops a recoverable task or hammers an unrecoverable one.
- **`Publish` is a method on `*Queue`, not a package function.** Tasks with an `AsyncDelay` are published from a background goroutine after sleeping; everything else is synchronous. `allowImmediate` lets unsigned, low-priority tasks skip storage and go straight to the in-memory buffer when there's room.
