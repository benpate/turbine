# queue_mongo

A MongoDB-backed `Storage` provider for [Turbine](../README.md). This is the production storage engine: it safely locks and dequeues tasks across any number of distributed queue workers. It implements the [`queue.Storage`](../queue/) interface.

```go
provider := queue_mongo.New(database, 32, 5) // database, lockQuantity, timeoutMinutes
q := queue.New(queue.WithStorage(provider))
```

## What matters here

- **Locking is timeout-based, not transactional.** `lockTasks` claims tasks by stamping a unique `lockId` and a future `timeoutDate` on rows whose `timeoutDate` has already passed. A worker that dies mid-task does not release its lock — the task simply becomes claimable again once its `timeoutDate` elapses (`timeoutMinutes`). Set `timeoutMinutes` longer than your slowest task, or healthy workers will have tasks stolen out from under them.
- **`New` takes a `*mongo.Database`, not a client or collection.** The collection names are fixed constants: `CollectionQueue` (`"Queue"`) for pending tasks and `CollectionLog` (`"QueueErrors"`) for permanently-failed tasks. Two queues sharing a database share those collections.
- **Every database call is wrapped in a 16-second timeout context** (`timeoutContext`) with a deferred `cancel()`. Keep that pattern when adding methods — a missing `cancel()` leaks the context, and an unbounded call can hang a worker.
- **`isDuplicateSignature` silently drops duplicates.** `SaveTask` returns `nil` (success) without writing when a task's `Signature` already exists in the queue. This is intentional de-duplication, not an error — callers cannot distinguish "saved" from "skipped as duplicate".
- **`lockQuantity` is the batch size per poll**, bounding how many tasks one worker pull locks at once. It is the mongo analogue of the queue's `bufferSize`; size it against worker throughput.
