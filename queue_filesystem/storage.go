package queue_filesystem

import (
	"encoding/json"
	"errors"
	"io/fs"
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

// SaveTask writes a task to the queue as a new spool file
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

	// RULE: 0600 — task arguments can carry sensitive data (IDs, hostnames,
	// application payloads), so spool files must not be world-readable.
	if err = os.WriteFile(filename, data, 0600); err != nil {
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

	// RULE: TaskIDs are self-generated ObjectID hex (see SaveTask).  Validating here
	// makes it impossible for a malformed ID to build a path outside the storage directory.
	if _, err := primitive.ObjectIDFromHex(taskID); err != nil {
		return derp.Wrap(err, location, "TaskID must be a valid ObjectID", taskID)
	}

	// Try to delete the task from the filesystem
	filename := storage.directory + "/" + taskID + ".json"
	if err := os.Remove(filename); err != nil {

		// RULE: A missing file is success, not an error.  GetTasks removes each
		// spool file as it hands the Task to a worker (this backend runs tasks
		// at-most-once), so the file is already gone by the time the Queue
		// reports the Task complete and calls DeleteTask.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return derp.Wrap(err, location, "Unable to delete task file", filename)
	}

	// Silence is acquiescence
	log.Trace().
		Str("location", location).
		Str("taskId", taskID).
		Msg("Task deleted.")

	return nil
}

// DeleteTaskBySignature removes a task from the queue by its signature
func (storage Storage) DeleteTaskBySignature(_ string) error {
	return derp.NotImplemented("queue_filesystem.DeleteTaskBySignature", "Filesystem storage cannot delete task by signature.")
}

// LogFailure reports a failed task.  This backend keeps no persistent error log.
func (storage Storage) LogFailure(task queue.Task) error {

	const location = "queue_filesystem.LogFailure"
	derp.Report(derp.Internal(location, "Failure executing task", task))

	return nil
}

// GetTasks claims and returns at most one task, removing its spool file
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

		// Use the full path for both reading and removing, so they cannot diverge
		path := storage.directory + "/" + filename

		// Read the first file in the directory.
		// #nosec G304 -- path is storage.directory + a name returned by ReadDir
		// of that same directory (filtered to .json); it never contains external input.
		file, err := os.ReadFile(path)

		if err != nil {
			return nil, derp.Wrap(err, location, "Unable to read task file", path)
		}

		// Unmarshal the file into a Task
		task := queue.Task{}
		if err := json.Unmarshal(file, &task); err != nil {
			return nil, derp.Wrap(err, location, "Unable to unmarshal task file", path)
		}

		// Remove the task from the directory, because we are about to execute it
		if err := os.Remove(path); err != nil {
			derp.Report(derp.Wrap(err, location, "Unable to delete task file", path))
		}

		// Success.  Return the single task.
		return []queue.Task{task}, nil
	}

	// If no valid files found, then return an empty slice
	return make([]queue.Task, 0), nil
}
