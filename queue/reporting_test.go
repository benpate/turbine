package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/benpate/derp"
	"github.com/stretchr/testify/require"
)

// recordingReporter is a derp Reporter that remembers every error it is handed
type recordingReporter struct {
	reported *[]error
}

// Report implements the derp.Reporter interface
func (reporter recordingReporter) Report(err error) {
	*reporter.reported = append(*reporter.reported, err)
}

// recordReports swaps derp's reporters for one that records, and restores them when the test ends
func recordReports(t *testing.T) *[]error {

	reported := make([]error, 0)
	original := derp.Plugins

	derp.Plugins = derp.ReporterList{recordingReporter{reported: &reported}}
	t.Cleanup(func() { derp.Plugins = original })

	return &reported
}

// TestReporting_ErrorIsRetriedQuietly confirms that a retry-able error is re-queued without a report
func TestReporting_ErrorIsRetriedQuietly(t *testing.T) {

	reported := recordReports(t)
	storage := &mockStorage{}
	consumer, _, _ := newRecordingConsumer(Error(errors.New("try again")))
	q := New(WithStorage(storage), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 0, RetryMax: 8}))
	require.Len(t, storage.saved, 1, "the task must still be re-queued")
	require.Empty(t, *reported)
}

// TestReporting_ExhaustedErrorIsReportedOnce confirms that a task out of retries is reported as a failure
func TestReporting_ExhaustedErrorIsReportedOnce(t *testing.T) {

	reported := recordReports(t)
	storage := &mockStorage{}
	consumer, _, _ := newRecordingConsumer(Error(errors.New("still broken")))
	q := New(WithStorage(storage), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x", RetryCount: 8, RetryMax: 8}))
	require.Len(t, storage.failures, 1)
	require.Len(t, *reported, 1)
}

// TestReporting_FailureIsReportedOnce confirms that a permanent failure is reported exactly once
func TestReporting_FailureIsReportedOnce(t *testing.T) {

	reported := recordReports(t)
	consumer, _, _ := newRecordingConsumer(Failure(errors.New("never")))
	q := New(WithStorage(&mockStorage{}), WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Len(t, *reported, 1)
}

// TestReporting_FailureWithoutStorageIsReportedOnce covers the memory-only queue, which has no failure log
func TestReporting_FailureWithoutStorageIsReportedOnce(t *testing.T) {

	reported := recordReports(t)
	consumer, _, _ := newRecordingConsumer(Failure(errors.New("never")))
	q := New(WithConsumers(consumer))

	require.NoError(t, q.consume(Task{TaskID: "abc", Name: "x"}))
	require.Len(t, *reported, 1)
}

// TestReporting_UnrecognizedTaskIsReportedOnce confirms that a task no Consumer recognizes is
// reported once, by name, and never with its Arguments (BUG-213)
func TestReporting_UnrecognizedTaskIsReportedOnce(t *testing.T) {

	const marker = "s3cretTaskArgument"

	reported := recordReports(t)
	ignoreConsumer := func(Task) Result { return Ignored() }
	q := New(WithStorage(&mockStorage{}), withConsumerFuncs(ignoreConsumer))

	task := Task{TaskID: "abc", Name: "RetiredTaskName", Arguments: map[string]any{"secret": marker}, Signature: marker}
	require.NoError(t, q.consume(task))
	require.Len(t, *reported, 1)

	// The report names the task...
	err := (*reported)[0]
	serialized, marshalErr := json.Marshal(err)
	require.NoError(t, marshalErr)
	require.Contains(t, string(serialized), "RetiredTaskName")

	// ...and never carries it, in any form a reporter might store
	require.NotContains(t, string(serialized), marker)
	require.NotContains(t, err.Error(), marker)
	require.NotContains(t, fmt.Sprintf("%+v", err), marker)
}
