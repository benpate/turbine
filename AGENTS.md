# turbine — Notes for AI Agents

Turbine is a distributed task queue with swappable storage back ends: the core [queue](queue/) package plus the [queue_mongo](queue_mongo/) (production, distributed) and [queue_filesystem](queue_filesystem/) (single-worker, development) storage providers. Start with the [README](README.md) and the per-package READMEs — they carry the operational detail; this file holds the contracts and foot-guns that cut across them.

## Delivery and consumer contracts

- **Delivery is at-least-once on the mongo backend — consumers must be idempotent.** Locking is timeout-based, not transactional: a worker that dies after executing a task but before `DeleteTask` leaves the record in place, and it becomes claimable again once its `timeoutDate` passes. Retries via `queue.Error` re-run the task too. Never write a consumer that breaks when its task runs twice.
- **The filesystem backend and Inline tasks are at-most-once, by design.** `queue_filesystem.GetTasks` deletes the spool file before the task runs, and `WithInline` tasks are never stored or retried — a crash mid-task loses them. Don't "fix" either into at-least-once; pick the mongo backend when redelivery matters.
- **`Error` means retryable, `Failure` means permanent.** `queue.Error(err)` re-queues with exponential backoff (`2^RetryCount` minutes) until `RetryCount` reaches `RetryMax`, then becomes a failure; `queue.Failure(err)` goes straight to the error log and the task is deleted. `queue.Requeue(delay)` marks THIS run successful and publishes a fresh copy (identity and counters reset) to run again. `queue.Ignored()` means "not my task" and passes it to the next registered consumer, so every consumer must ignore names it doesn't own. Any other status is converted to an immediate `Failure` by `dispatch` — that backstop is deliberate so a new status can never silently pass as success.
- **Task argument keys are a persisted wire format.** Arguments are written to storage as-is and drain over hours or days, so renaming a key strands already-queued tasks. A consumer renaming an argument must read the new key with a fallback to the old one until the persistent queue has fully drained; only then may the fallback be removed. Argument values must round-trip through BSON/JSON (`mapof.Any`) — plain data only, never closures or live pointers.

## Publish and shutdown

- **`Publish` runs the `PreProcessor` once, then routes the task itself.** `requeueTask` re-enters `Publish` on purpose: a requeued task gets a fresh identity, so it is a new publish and earns a fresh pre-processing pass.
- **There is no in-memory delay, and there must not be one again.** `WithAsyncDelay` — a timer goroutine that re-published the task after N milliseconds — was removed on 2026-09-17. Its only consumer was one Emissary call site, where a one-second sleep let a request transaction commit before a priority-16 task ran immediately and tried to load the row; that race is what Emissary's `tools/postcommit` now closes properly, so the sleep had been dead weight since 2026-07-13. A delay worth having is a durable one: `Schedule` and `WithDelaySeconds` write a future `StartDate` to the storage provider and survive a restart.
- **Every process-local goroutine registers with `q.workers` before launch, and every `q.buffer` send selects on `q.done`.** Workers and inline runners both follow this pattern ([queue-worker.go](queue/queue-worker.go), [queue-inline.go](queue/queue-inline.go)). A bare `q.buffer <- task` deadlocks against `Stop` when the buffer is full, and an untracked goroutine outlives shutdown. `Stop` is idempotent via `sync.Once`; publishing an Inline task after `Stop` errors fast, while storage-backed publishes still write (another process or the next restart picks them up — that asymmetry is intentional).
- **The `backoff` exponent clamp (27) is important.** `2^28` minutes overflows `time.Duration` to a negative value, which schedules the retry in the past and hot-loops the queue. Keep the clamp even though no real retry policy ever reaches it.

## Signatures and priority

- **`queue_mongo.isDuplicateSignature` must exclude the task's own `_id`.** `SaveTask` is also how a retry writes its updated counters back; if the duplicate check matches the task's own record, the write-back is silently discarded, `RetryCount` never advances, and the task re-runs every `timeoutMinutes` forever. The `$ne: taskID` filter is the fix — never remove it.
- **Signature duplicates are dropped silently, and a signed task never runs immediately.** `SaveTask` returns `nil` without writing when another task holds the signature (callers cannot tell "saved" from "skipped"), `allowImmediate` refuses signed tasks so the dedup check always sees storage, and `WithSignature` + `WithInline` is rejected at publish (nothing stored to dedup against). `Queue.Delete(signature)` requires a storage provider and is unimplemented on the filesystem backend.
- **Lower priority numbers run first, and both mongo sorts must stay identical.** `pickTasks` and `GetTasks` each sort priority-ascending then `startDate`; if they diverge, a locked batch dispatches out of priority order. Tasks with priority ≤ `runImmediatePriority` (default 16) skip storage into the in-memory buffer when there is room.

## Deliberate non-fixes

- **`Option` vs `QueueOption` naming stays as-is** — reviewed and ruled not a problem.
- **`queue_filesystem.SaveTask` always mints a new `TaskID`** — reviewed and ruled not a problem; a retry write-back becomes a new spool file.
- **`queue_filesystem.DeleteTask` treats a missing file as success** — `GetTasks` already removed the spool file when it handed the task to a worker, so the completion-path delete always finds it gone.
- **A memory-only queue retries with no backoff delay** — with no storage, `onTaskError` puts the task straight back on the buffer. Retry delays and future scheduling require a storage provider.

## Consumers of this library

- **Emissary layers `tools/postcommit` on top** so tasks enqueued inside a database transaction are published only after commit ("the queue is the outbox"). Changes to `Publish` semantics ripple into that gate.
