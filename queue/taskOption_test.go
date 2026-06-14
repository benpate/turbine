package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewTask_Defaults(t *testing.T) {

	task := NewTask("myTask", map[string]any{"a": 1})

	require.Equal(t, "myTask", task.Name)
	require.Equal(t, 1, task.Arguments["a"])
	require.Equal(t, -1, task.Priority)   // unset sentinel
	require.Equal(t, -1, task.RetryMax)   // unset sentinel
	require.Equal(t, 0, task.RetryCount)
	require.Equal(t, 0, task.AsyncDelay)
	require.Equal(t, task.CreateDate, task.StartDate)
}

func TestWithPriority(t *testing.T) {
	task := NewTask("x", nil, WithPriority(5))
	require.Equal(t, 5, task.Priority)
}

func TestWithAsyncDelay(t *testing.T) {
	task := NewTask("x", nil, WithAsyncDelay(250))
	require.Equal(t, 250, task.AsyncDelay)
}

func TestWithRetryMax(t *testing.T) {
	task := NewTask("x", nil, WithRetryMax(3))
	require.Equal(t, 3, task.RetryMax)
}

func TestWithSignature(t *testing.T) {
	task := NewTask("x", nil, WithSignature("unique-sig"))
	require.Equal(t, "unique-sig", task.Signature)
}

func TestWithStartTime(t *testing.T) {
	when := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	task := NewTask("x", nil, WithStartTime(when))
	require.Equal(t, when.Unix(), task.StartDate)
}

func TestWithDelaySeconds(t *testing.T) {
	before := time.Now().Add(9 * time.Second).Unix()
	task := NewTask("x", nil, WithDelaySeconds(10))
	after := time.Now().Add(11 * time.Second).Unix()

	require.GreaterOrEqual(t, task.StartDate, before)
	require.LessOrEqual(t, task.StartDate, after)
}

func TestWithDelayMinutes(t *testing.T) {
	expected := time.Now().Add(5 * time.Minute).Unix()
	task := NewTask("x", nil, WithDelayMinutes(5))
	require.InDelta(t, expected, task.StartDate, 2)
}

func TestWithDelayHours(t *testing.T) {
	expected := time.Now().Add(2 * time.Hour).Unix()
	task := NewTask("x", nil, WithDelayHours(2))
	require.InDelta(t, expected, task.StartDate, 2)
}

func TestTask_Delay(t *testing.T) {
	task := NewTask("x", nil)
	expected := time.Now().Add(30 * time.Minute).Unix()
	task.Delay(30 * time.Minute)
	require.InDelta(t, expected, task.StartDate, 2)
}

func TestNewTask_MultipleOptions(t *testing.T) {
	task := NewTask("x", nil,
		WithPriority(2),
		WithRetryMax(4),
		WithSignature("sig"),
	)
	require.Equal(t, 2, task.Priority)
	require.Equal(t, 4, task.RetryMax)
	require.Equal(t, "sig", task.Signature)
}
