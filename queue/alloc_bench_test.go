package queue

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// quietLogs silences zerolog for a benchmark, so the numbers show what the queue itself
// allocates rather than what logging does
func quietLogs(b *testing.B) {
	previous := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	b.Cleanup(func() { zerolog.SetGlobalLevel(previous) })
}

func BenchmarkNewTask(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewTask("taskName", nil)
	}
}

func BenchmarkNewTask_WithOptions(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewTask("taskName", nil, WithPriority(4), WithRetryMax(2))
	}
}

func BenchmarkPublish_Buffer(b *testing.B) {
	quietLogs(b)

	q := New(WithBufferSize(b.N+8), withConsumerFuncs(func(Task) Result { return Success() }))
	task := NewTask("taskName", nil)
	task.Priority = 1
	task.RetryMax = 1

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := q.Publish(task); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConsume_Success(b *testing.B) {
	quietLogs(b)

	q := New(withConsumerFuncs(func(Task) Result { return Success() }))
	task := Task{TaskID: "abc", Name: "taskName", RetryMax: 8}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := q.consume(task); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConsume_SuccessWithLogging(b *testing.B) {

	// Trace is zerolog's DEFAULT level, so this is what an application that never configures
	// logging pays per task.  Writing to io.Discard measures the formatting, not the terminal.
	previousLogger := log.Logger
	previousLevel := zerolog.GlobalLevel()
	log.Logger = zerolog.New(io.Discard)
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	b.Cleanup(func() {
		log.Logger = previousLogger
		zerolog.SetGlobalLevel(previousLevel)
	})

	q := New(withConsumerFuncs(func(Task) Result { return Success() }))
	task := Task{TaskID: "abc", Name: "taskName", RetryMax: 8}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := q.consume(task); err != nil {
			b.Fatal(err)
		}
	}
}
