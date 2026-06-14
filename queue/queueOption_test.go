package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew_Defaults(t *testing.T) {

	q := New()

	require.Equal(t, 16, q.workerCount)
	require.Equal(t, 32, q.bufferSize)
	require.Equal(t, 16, q.defaultPriority)
	require.Equal(t, 16, q.runImmediatePriority)
	require.Equal(t, 8, q.defaultRetryMax)
	require.True(t, q.pollStorage)
	require.NotNil(t, q.buffer)
	require.NotNil(t, q.done)
	require.Nil(t, q.storage)
}

func TestWithConsumers(t *testing.T) {

	consumer := func(string, map[string]any) Result { return Success() }

	q := New(WithConsumers(consumer, consumer))
	require.Equal(t, 2, len(q.consumers))
}

func TestWithStorage(t *testing.T) {
	storage := &mockStorage{}
	q := New(WithStorage(storage))
	require.Equal(t, storage, q.storage)
}

func TestWithWorkerCount(t *testing.T) {
	q := New(WithWorkerCount(4))
	require.Equal(t, 4, q.workerCount)
}

func TestWithBufferSize(t *testing.T) {
	q := New(WithBufferSize(64))
	require.Equal(t, 64, q.bufferSize)
	require.Equal(t, 64, cap(q.buffer)) // buffer is sized after options apply
}

func TestWithPollStorage(t *testing.T) {
	q := New(WithPollStorage(false))
	require.False(t, q.pollStorage)
}

func TestWithDefaultPriority(t *testing.T) {
	q := New(WithDefaultPriority(99))
	require.Equal(t, 99, q.defaultPriority)
}

func TestWithRunImmediatePriority(t *testing.T) {
	q := New(WithRunImmediatePriority(3))
	require.Equal(t, 3, q.runImmediatePriority)
}

func TestWithDefaultRetryMax(t *testing.T) {
	q := New(WithDefaultRetryMax(2))
	require.Equal(t, 2, q.defaultRetryMax)
}

func TestWithPreProcessor(t *testing.T) {
	preProcessor := func(*Task) error { return nil }
	q := New(WithPreProcessor(preProcessor))
	require.NotNil(t, q.preProcessor)
}
