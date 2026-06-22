package queue_filesystem

import (
	"os"
	"testing"

	"github.com/benpate/turbine/queue"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	storage := New("/tmp/some-dir")
	require.Equal(t, "/tmp/some-dir", storage.directory)
}

func TestSaveTask(t *testing.T) {

	dir := t.TempDir()
	storage := New(dir)

	require.NoError(t, storage.SaveTask(queue.NewTask("myTask", map[string]any{"k": "v"})))

	// Exactly one JSON file was written
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Equal(t, 1, len(files))
	require.Contains(t, files[0].Name(), ".json")
}

func TestSaveTask_BadDirectory(t *testing.T) {

	// Writing into a non-existent directory fails
	storage := New("/nonexistent-directory-xyz")
	require.Error(t, storage.SaveTask(queue.NewTask("x", nil)))
}

func TestGetTasks(t *testing.T) {

	dir := t.TempDir()
	storage := New(dir)

	require.NoError(t, storage.SaveTask(queue.NewTask("hello", map[string]any{"a": 1})))

	tasks, err := storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	require.Equal(t, "hello", tasks[0].Name)

	// GetTasks must remove the task file, so a second call returns nothing
	// (otherwise the same task would be re-executed on every poll).
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Equal(t, 0, len(files))

	tasks, err = storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))
}

func TestGetTasks_EmptyDirectory(t *testing.T) {

	storage := New(t.TempDir())

	tasks, err := storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))
}

func TestGetTasks_IgnoresNonJSON(t *testing.T) {

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/notes.txt", []byte("ignore me"), 0644))

	storage := New(dir)
	tasks, err := storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))
}

func TestGetTasks_MissingDirectory(t *testing.T) {

	storage := New("/nonexistent-directory-xyz")
	_, err := storage.GetTasks()
	require.Error(t, err)
}

func TestGetTasks_InvalidJSON(t *testing.T) {

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/bad.json", []byte("not json"), 0644))

	storage := New(dir)
	_, err := storage.GetTasks()
	require.Error(t, err)
}

func TestDeleteTask(t *testing.T) {

	dir := t.TempDir()
	storage := New(dir)

	// Save a task, then locate its file to learn the generated TaskID
	require.NoError(t, storage.SaveTask(queue.NewTask("x", nil)))
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Equal(t, 1, len(files))

	taskID := files[0].Name()[:len(files[0].Name())-len(".json")]

	require.NoError(t, storage.DeleteTask(taskID))

	files, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Equal(t, 0, len(files))
}

func TestDeleteTask_EmptyID(t *testing.T) {
	// An empty taskID represents an in-memory task; deletion is a no-op
	storage := New(t.TempDir())
	require.NoError(t, storage.DeleteTask(""))
}

func TestDeleteTask_Missing(t *testing.T) {
	storage := New(t.TempDir())
	require.Error(t, storage.DeleteTask("does-not-exist"))
}

func TestDeleteTaskBySignature_NotImplemented(t *testing.T) {
	storage := New(t.TempDir())
	require.Error(t, storage.DeleteTaskBySignature("sig"))
}

func TestLogFailure(t *testing.T) {
	// LogFailure reports the error internally but always returns nil
	storage := New(t.TempDir())
	require.NoError(t, storage.LogFailure(queue.NewTask("x", nil)))
}

func TestStorage_ImplementsInterface(_ *testing.T) {
	var _ queue.Storage = Storage{}
}
