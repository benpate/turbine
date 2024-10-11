# Turbine

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
    queue.WithStorage(), // optional storage adapter persists tasks 
    queue.WithTimeout()
)
```


## Consuming Tasks from the Queue


## Mongo Storage Provider