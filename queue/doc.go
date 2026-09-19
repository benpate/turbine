// Package queue is a distributed task queue with swappable storage back ends.
//
// A Queue accepts Tasks, hands them to a pool of workers, and retries the ones that fail,
// backing off exponentially until they succeed or run out of attempts.  Tasks are held by a
// Storage provider -- queue_mongo in production, queue_filesystem for development -- or kept
// in memory when no provider is configured.
//
// The work itself is done by Consumers.  Each is offered a Task in turn and returns Ignored()
// for names it does not own, so several Consumers can share one Queue.  Whichever Consumer
// handles a Task is also told how it turned out, through OnSuccess, OnError and OnFailure.
//
// Delivery is at-least-once on the mongo backend, so a Consumer must be safe to run twice.
package queue
