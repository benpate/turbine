package queue

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// newPublishRecorder returns a Consumer that records every Task handed to OnPublish, and the
// slice it records into
func newPublishRecorder() (recordingConsumer, *[]Task) {

	events := make([]string, 0)
	tasks := make([]Task, 0)
	seen := make([]Task, 0)

	consumer := recordingConsumer{
		result: Success(),
		events: &events,
		tasks:  &tasks,
		onPublish: func(task *Task) error {
			seen = append(seen, *task)
			return nil
		},
	}

	return consumer, &seen
}

// TestOnPublish_FiresForPublish confirms that every published Task passes through OnPublish
func TestOnPublish_FiresForPublish(t *testing.T) {

	consumer, seen := newPublishRecorder()
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.Publish(NewTask("SendEmail", map[string]any{"to": "a@b.test"})))

	require.Len(t, *seen, 1)
	require.Equal(t, "SendEmail", (*seen)[0].Name)
	require.Equal(t, "a@b.test", (*seen)[0].Arguments.GetString("to"))
}

// TestOnPublish_FiresForSchedule confirms that a scheduled Task passes through the same rules
func TestOnPublish_FiresForSchedule(t *testing.T) {

	consumer, seen := newPublishRecorder()
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.Schedule(NewTask("SendEmail", map[string]any{}), 0))
	require.Len(t, *seen, 1)
}

// TestOnPublish_SeesUnsetSentinels pins the ORDER of the publish-time rules: a hook runs before
// the Queue's defaults, so it can still tell "unset" from "deliberately set to the default".
func TestOnPublish_SeesUnsetSentinels(t *testing.T) {

	consumer, seen := newPublishRecorder()
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.Publish(NewTask("x", map[string]any{})))

	require.Len(t, *seen, 1)
	require.Equal(t, -1, (*seen)[0].Priority)
	require.Equal(t, -1, (*seen)[0].RetryMax)
}

// TestOnPublish_CanModifyTheTask confirms that a hook's choices survive, and that the Queue's
// defaults do not overwrite them
func TestOnPublish_CanModifyTheTask(t *testing.T) {

	events := make([]string, 0)
	tasks := make([]Task, 0)

	consumer := recordingConsumer{
		result: Success(),
		events: &events,
		tasks:  &tasks,
		onPublish: func(task *Task) error {
			task.Priority = 1024
			task.RetryMax = 2
			return nil
		},
	}

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithConsumers(consumer))

	require.NoError(t, q.Publish(NewTask("x", map[string]any{})))

	require.Len(t, storage.saved, 1)
	require.Equal(t, 1024, storage.saved[0].Priority)
	require.Equal(t, 2, storage.saved[0].RetryMax)
}

// TestOnPublish_CanRejectTheTask confirms that a hook's error stops the Task from being queued
func TestOnPublish_CanRejectTheTask(t *testing.T) {

	events := make([]string, 0)
	tasks := make([]Task, 0)

	consumer := recordingConsumer{
		result: Success(),
		events: &events,
		tasks:  &tasks,
		onPublish: func(*Task) error {
			return errors.New("this task is not welcome here")
		},
	}

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithConsumers(consumer))

	require.Error(t, q.Publish(NewTask("x", map[string]any{})))
	require.Error(t, q.Schedule(NewTask("x", map[string]any{}), 0))

	// Nothing reached storage
	require.Empty(t, storage.saved)
}

// TestOnPublish_EveryConsumerInOrder confirms that all consumers see the Task, in registration
// order, each one seeing what the one before it did
func TestOnPublish_EveryConsumerInOrder(t *testing.T) {

	order := make([]string, 0)

	first := recordingConsumer{
		result: Ignored(), events: &[]string{}, tasks: &[]Task{},
		onPublish: func(task *Task) error {
			order = append(order, "first")
			task.Priority = 100
			return nil
		},
	}

	second := recordingConsumer{
		result: Success(), events: &[]string{}, tasks: &[]Task{},
		onPublish: func(task *Task) error {
			order = append(order, "second:"+strconv.Itoa(task.Priority))
			return nil
		},
	}

	q := New(WithStorage(&mockStorage{}), WithConsumers(first, second))
	require.NoError(t, q.Publish(NewTask("x", map[string]any{})))

	require.Equal(t, []string{"first", "second:100"}, order)
}

// TestPrepareTask_ScheduleAppliesDefaults is the regression test for a Task that reached storage
// carrying NewTask's "unset" sentinels.  A RetryMax of -1 makes onTaskError compute `0 >= -1`,
// which abandoned the Task on its very first failure; a Priority of -1 sorted it ahead of
// everything else, because lower numbers run first.
func TestPrepareTask_ScheduleAppliesDefaults(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage))

	task := NewTask("x", map[string]any{})
	require.Equal(t, -1, task.Priority, "NewTask writes the unset sentinel")
	require.Equal(t, -1, task.RetryMax, "NewTask writes the unset sentinel")

	require.NoError(t, q.Schedule(task, 0))

	require.Len(t, storage.saved, 1)
	require.Equal(t, 16, storage.saved[0].Priority)
	require.Equal(t, 8, storage.saved[0].RetryMax)
}

// TestPrepareTask_PreProcessorRunsForSchedule confirms that the centralized rules a PreProcessor
// defines are not skipped by the scheduling path
func TestPrepareTask_PreProcessorRunsForSchedule(t *testing.T) {

	ran := false
	storage := &mockStorage{}

	q := New(WithStorage(storage), WithPreProcessor(func(task *Task) error {
		ran = true
		task.Priority = 1024
		return nil
	}))

	require.NoError(t, q.Schedule(NewTask("x", map[string]any{}), 0))

	require.True(t, ran)
	require.Equal(t, 1024, storage.saved[0].Priority)
}

// TestPrepareTask_PreProcessorCanRejectSchedule confirms that a rejected Task is not scheduled
func TestPrepareTask_PreProcessorCanRejectSchedule(t *testing.T) {

	storage := &mockStorage{}
	q := New(WithStorage(storage), WithPreProcessor(func(*Task) error {
		return errors.New("nope")
	}))

	require.Error(t, q.Schedule(NewTask("x", map[string]any{}), 0))
	require.Empty(t, storage.saved)
}
