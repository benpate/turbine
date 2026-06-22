# queue_filesystem

A filesystem-backed `Storage` provider for [Turbine](../README.md), storing each task as a JSON file in a directory. It implements the [`queue.Storage`](../queue/) interface, but is a **simplified, single-worker** engine intended for development and small/local deployments — not the distributed production path (see [queue_mongo](../queue_mongo/) for that).

```go
provider := queue_filesystem.New("/path/to/queue/dir")
q := queue.New(queue.WithStorage(provider))
```

## What matters here

- **Not safe for concurrent workers.** There is no locking: `GetTasks` returns the first JSON file it finds and deletes it, so two workers reading the same directory can grab and run the same task. Run a single worker against a given directory.
- **`GetTasks` returns at most one task per call**, in no guaranteed order (it depends on `os.ReadDir` ordering). Don't rely on FIFO or priority ordering with this backend.
- **`DeleteTaskBySignature` is unimplemented** — it returns `derp.NotImplemented`. Signature-based de-duplication (which the mongo backend supports) is *not* available here, so `Queue.Delete` will error against this provider.
- **`LogFailure` does not persist.** A permanently-failed task is only reported via `derp.Report`, not written to disk — failures are not durably recorded by this backend.
