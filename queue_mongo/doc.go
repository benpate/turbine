// Package queue_mongo stores turbine Tasks in MongoDB, for production queues shared by several
// workers.
//
// Tasks are claimed in batches: lockTasks stamps a unique lockId and a future timeoutDate onto
// rows whose lease has already expired, and GetTasks returns the rows carrying that lockId.
// Locking is timeout-based rather than transactional, so a worker that dies mid-task never
// releases its claim -- the row becomes claimable again once its timeoutDate passes.
//
// Delivery is therefore at-least-once, and a Consumer must be safe to run twice.  Tasks are
// dispatched by priority first (lower numbers run first), then by start date, and a Task whose
// StartDate is still in the future is not claimed at all.
//
// New takes the database, the number of Tasks to claim per batch, and the lease length in
// minutes.
package queue_mongo
