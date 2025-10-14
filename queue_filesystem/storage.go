package queue_filesystem

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/benpate/derp"
	"github.com/benpate/turbine/queue"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Storage implements a simplified queue Storage interface using a filesystem.
// This storage engine does not support duplicate checking.  It returns a single
// task at a time, probably in order of creation time (but no guarantees are made).
// This storage engine is not safe for concurrent access by multiple workers.
type Storage struct {
	directory string // The filesystem directory to read/write
}

// New returns a fully initialized Storage object
func New(directory string) Storage {

	return Storage{
		directory: directory,
	}
}

// SaveTask adds/updates a task to the queue
func (storage Storage) SaveTask(task queue.Task) error {

	const location = "queue_filesystem.SaveTask"

	log.Trace().
		Str("location", location).
		Str("task", task.Name).
		Msg("Saving Task...")

	task.TaskID = primitive.NewObjectID().Hex()
	filename := storage.directory + "/" + task.TaskID + ".json"

	// Marshal the task into JSON
	data, err := json.Marshal(task)

	if err != nil {
		return derp.Wrap(err, location, "Unable to marshal task")
	}

	if err = os.WriteFile(filename, data, 0644); err != nil {
		return derp.Wrap(err, location, "Unable to write task file", filename)
	}

	log.Trace().
		Str("location", location).
		Str("task", task.Name).
		Msg("Task saved.")

	// Silence is golden
	return nil
}

// DeleteTask removes a task from the queue
func (storage Storage) DeleteTask(taskID string) error {

	const location = "queue_filesystem.DeleteTask"

	log.Trace().
		Str("location", location).
		Str("taskId", taskID).
		Msg("Deleting task from queue...")

	// If the taskID is empty, then this is an in-memory task, so there's nothing to do
	if taskID == "" {
		return nil
	}

	// Try to delete the task from the filesystem
	filename := storage.directory + "/" + taskID + ".json"
	if err := os.Remove(filename); err != nil {
		return derp.ReportAndReturn(derp.Wrap(err, location, "Unable to delete task file", filename))
	}

	// Silence is acquiescence
	log.Trace().
		Str("location", location).
		Str("taskId", taskID).
		Msg("Task deleted.")

	return nil
}

func (storage Storage) DeleteTaskBySignature(signature string) error {
	return derp.NotImplemented("queue_filesystem.DeleteTaskBySignature", "Filesystem storage cannot delete task by signature.")
}

// LogFailure adds a task to the error log
func (storage Storage) LogFailure(task queue.Task) error {

	const location = "queue_filesystem.LogFailure"
	derp.Report(derp.InternalError(location, "Failure executing task", task))

	return nil
}

// GetTasks returns all tasks that are currently locked by this worker
func (storage Storage) GetTasks() ([]queue.Task, error) {

	const location = "queue_filesystem.GetTasks"

	// Read all files in the task directory
	files, err := os.ReadDir(storage.directory)
	if err != nil {
		return nil, derp.Wrap(err, location, "Unable to read task directory", storage.directory)
	}

	// Check each file in the directory
	for _, entry := range files {

		// If this is not a JSON file, then skip it
		filename := entry.Name()
		if !strings.HasSuffix(filename, ".json") {
			continue
		}

		// Read the first file in the directory
		file, err := os.ReadFile(storage.directory + "/" + filename)

		if err != nil {
			return nil, derp.Wrap(err, location, "Unable to read task file", filename)
		}

		// Unmarshal the file into a Task
		task := queue.Task{}
		if err := json.Unmarshal(file, &task); err != nil {
			return nil, derp.Wrap(err, location, "Unable to unmarshal task file", filename)
		}

		// Remove the task from the directory (because we're gonna execute it nao)
		if err := os.Remove(filename); err != nil {
			derp.Report(derp.Wrap(err, location, "Unable to delete task file", filename))
		}

		// Success.  Return the single task.
		return []queue.Task{task}, nil
	}

	// If no valid files found, then return an empty slice
	return make([]queue.Task, 0), nil
}
