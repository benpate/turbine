package queue_filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/benpate/turbine/queue"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestDeleteTask_InvalidID pins the ObjectID guard: a malformed TaskID is rejected before any
// filesystem path is built, so it can never reach outside the storage directory
func TestDeleteTask_InvalidID(t *testing.T) {

	directory := t.TempDir()
	storage := New(directory)

	// Plant a file OUTSIDE the storage directory that a traversal would reach
	outside := filepath.Join(directory, "..", "escape.json")
	require.NoError(t, os.WriteFile(outside, []byte("{}"), 0600))

	t.Cleanup(func() {
		_ = os.Remove(outside) // best-effort cleanup; TempDir removal handles the rest
	})

	// Non-ObjectID TaskIDs are rejected before any path is built
	require.Error(t, storage.DeleteTask("../escape"))
	require.Error(t, storage.DeleteTask("not-an-objectid"))

	// The traversal target is untouched
	_, err := os.Stat(outside)
	require.NoError(t, err, "traversal target must still exist")
}

// TestDeleteTask_AlreadyRemoved pins the completion path: the DeleteTask that follows a
// successful run finds nothing to remove, and that is success
func TestDeleteTask_AlreadyRemoved(t *testing.T) {

	// GetTasks removes each spool file as it hands the Task to a worker.  Treating the missing
	// file as an error made every completed task report a spurious 500.

	storage := New(t.TempDir())
	require.NoError(t, storage.SaveTask(queue.NewTask("hello", map[string]any{})))

	// GetTasks hands back the task AND removes its file
	tasks, err := storage.GetTasks()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.NotEmpty(t, tasks[0].TaskID, "task read back must carry its TaskID")

	// This is what queue.onTaskSuccess does when the task completes
	require.NoError(t, storage.DeleteTask(tasks[0].TaskID))

	// A second delete of the same (missing) task is equally harmless
	require.NoError(t, storage.DeleteTask(tasks[0].TaskID))
}

// TestDeleteTask_RealErrorStillReported confirms the missing-file tolerance did not swallow
// genuine filesystem errors
func TestDeleteTask_RealErrorStillReported(t *testing.T) {

	// A directory in place of the task file cannot be removed with os.Remove, and that error
	// must still surface.

	directory := t.TempDir()
	storage := New(directory)

	taskID := primitive.NewObjectID().Hex()

	// Plant a NON-EMPTY directory where the task file would live
	path := filepath.Join(directory, taskID+".json")
	require.NoError(t, os.Mkdir(path, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(path, "blocker.txt"), []byte("x"), 0600))

	require.Error(t, storage.DeleteTask(taskID))
}
