# Turbine

<img src="https://github.com/benpate/turbine/raw/main/meta/banner.webp" style="width:100%; display:block; margin-bottom:20px;"  alt="Oil painting titled: Dutch landscape with windmills, by Gerard Delfgaauw (1882-1947)">


[![GoDoc](https://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://pkg.go.dev/github.com/benpate/turbine)
[![Version](https://img.shields.io/github/v/release/benpate/turbine?include_prereleases&style=flat-square&color=brightgreen)](https://github.com/benpate/turbine/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/benpate/turbine/go.yml?style=flat-square)](https://github.com/benpate/turbine/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/benpate/turbine?style=flat-square)](https://goreportcard.com/report/github.com/benpate/turbine)
[![Codecov](https://img.shields.io/codecov/c/github/benpate/turbine.svg?style=flat-square)](https://codecov.io/gh/benpate/turbine)

## Fast, distributed message queue for Golang and MongoDB

There are many, many message queue tools. Perhaps you should use one of those instead.  But Turbine fills a unique niche in that it:

1. is a distributed queue that can be shared between several producer and consumer servers simultaneously
2. supports swappable storage providers, with the first provider being MongoDB.
6. supports fast, in-memory queues using Golang channels
4. can retry failed jobs (with exponential backoff)
5. can schedule jobs to in the future

## Pushing Tasks to the Queue

```go
// Create a Task with any parameters
task := queue.NewTask(
    "TaskName" // Task Name
    map[string]any{ // Parameters
        "foo": "bar",
        "baz": 42,
    }
)

// Publish the task to the Queue (this will happen quickly)
if err := queue.Publish(task); err != nil {
    // only errors related to queuing the task
}
```

## Run the Queue

```go
// Create and start a queue
q := queue.New(
    queue.WithConsumers(), // one or more "consumer" functions (below)
    queue.WithStorage(),   // optional storage adapter persists tasks 
    queue.WithTimeout(),   // other configs, like timeouts, retries, etc
)
```

## Consuming Tasks from the Queue

When the turbine queue receives a task, it tries to execute it using one or more `Consumer`
functions, which have the following signature:

```go
func Consumer(name string, args map[string]any) (bool, error)
```

It is the consumer's job to identify the task by name, and decide if it can 
execute it or not.  You can configure any number of consumer functions, so if 
a task is not recognized, it is passed to the next consumer until a match is found.

If the consumer DOES recognize the Task, then it executes the job and returns TRUE,
along with an error result (or nil if the task was successful).

When a task return an error, it is re-queued according to Turbine's exponential backoff logic, and will be re-run at some point in the future.

## Mongo Storage Provider

Turbine is built to support pluggable storage providers, so that any datastore can be used to manage queued tasks.

Currently, there is a single storage provider for Mongodb, which safely queues and dequeues tasks for any number of distributed queue workers.  However, [storage providers implement a simple interface](https://pkg.go.dev/github.com/benpate/turbine@v0.1.0/queue#Storage), so it is simple to create a new storage provider for any back end that you want to use.  

**IMPORTANT**: If you do not use a storage provider, the Turbine queue will still work, but will only work in memory.  This means that items cannot be queued for future dates, and will not have a retry delay.

To initialize the storage provider, use the following code:

```go
import (
    "github.com/benpate/turbine/queue"
    "github.com/benpate/turbine/queue_mongo"
)

// Create a MongoDB database connection
connection := mongo.Connect(...) 

// Pass the DB connection into the storage provider
provider := queue_mongo.New(connection)

// Initialize the queue with the storage provider
q := queue.New(queue.WithStorage(provider))
```
