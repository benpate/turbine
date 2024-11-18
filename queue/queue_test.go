package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {

	consumer := func(name string, args map[string]any) Result {
		return Success()
	}

	q := New(WithConsumers(consumer))

	for i := 0; i < 1000; i++ {
		require.Nil(t, q.Publish(NewTask("", nil)))
	}
}

func TestStop(t *testing.T) {

	q := New()
	q.Stop()
}
