package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {

	consumer := func(_ string, _ map[string]any) Result {
		return Success()
	}

	q := New(WithConsumers(consumer))

	// Start the workers so they drain the buffer. Without this, publishing more
	// than bufferSize tasks to a nil-storage queue deadlocks on a full channel.
	q.Start()
	defer q.Stop()

	for i := 0; i < 1000; i++ {
		require.Nil(t, q.Publish(NewTask("", nil)))
	}
}

func TestStop(_ *testing.T) {

	q := New()
	q.Stop()
}
