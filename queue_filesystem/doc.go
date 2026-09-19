// Package queue_filesystem stores turbine Tasks as JSON files in a directory, for development
// and single-worker use.
//
// Each Task becomes its own spool file, written 0600 because task arguments can carry sensitive
// data.  GetTasks returns one Task at a time and deletes its file before handing it over, so
// delivery is at-most-once: a worker that dies mid-task loses it.  Use queue_mongo when
// redelivery matters.
//
// This backend ignores Priority and StartDate, so Tasks do not run in priority order and
// Schedule does not defer them.  It has no signature support -- DeleteTaskBySignature returns
// NotImplemented -- and it is not safe for concurrent workers.
//
// New takes the directory that holds the spool files.
package queue_filesystem
