package queue_filesystem

import (
	"os"
	"testing"
	"unicode/utf8"

	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/turbine/queue"
	"github.com/stretchr/testify/require"
)

// FuzzStorageRoundTrip confirms that a Task saved to filesystem storage comes back from
// GetTasks with its name and arguments intact
func FuzzStorageRoundTrip(f *testing.F) {

	// This drives the real persistence path end to end: JSON marshal, file write, read,
	// unmarshal.

	f.Add("taskName", "argKey", "argValue")
	f.Add("", "", "")
	f.Add("emoji-🜂", "control\x00byte", "quote\"and\\slash")

	f.Fuzz(func(t *testing.T, name, argKey, argValue string) {

		// Tasks are persisted as JSON, which only round trips valid UTF-8 unchanged
		// (see queue.FuzzTaskJSONRoundTrip). Skip inputs the std library mangles.
		if !utf8.ValidString(name) || !utf8.ValidString(argKey) || !utf8.ValidString(argValue) {
			t.Skip()
		}

		storage := New(t.TempDir())

		// Save a single task into the (empty) storage directory
		require.NoError(t, storage.SaveTask(queue.NewTask(name, mapof.Any{argKey: argValue})))

		// GetTasks must return the task we saved, unchanged
		tasks, err := storage.GetTasks()
		require.NoError(t, err)
		require.Equal(t, 1, len(tasks))
		require.Equal(t, name, tasks[0].Name)
		require.Equal(t, argValue, tasks[0].Arguments.GetString(argKey))

		// GetTasks removes the file as it reads, so the queue cannot be drained twice.
		tasks, err = storage.GetTasks()
		require.NoError(t, err)
		require.Equal(t, 0, len(tasks))
	})
}

// FuzzGetTasks_ArbitraryBytes confirms that GetTasks never panics on arbitrary file contents
func FuzzGetTasks_ArbitraryBytes(f *testing.F) {

	// Files in the queue directory are untrusted bytes once they hit disk, so a malformed one
	// must produce a clean error or be skipped, never a crash.

	f.Add([]byte(`{"name":"valid"}`))
	f.Add([]byte("not json at all"))
	f.Add([]byte(""))
	f.Add([]byte(`{"name":`)) // truncated JSON

	f.Fuzz(func(t *testing.T, data []byte) {

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(dir+"/fuzz.json", data, 0644))

		storage := New(dir)

		// The only contract under fuzzing is "does not panic". Valid JSON returns a
		// task; invalid JSON returns an error. Either outcome is acceptable.
		_, _ = storage.GetTasks()
	})
}
