package queue_mongo

import (
	"context"
	"testing"
	"time"

	"github.com/benpate/turbine/queue"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// testStorage returns a Storage backed by a uniquely-named test database, dropped on cleanup.
// The test is skipped, not failed, when MongoDB is unreachable.
func testStorage(t *testing.T, lockQuantity int, timeoutMinutes int) Storage {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// directConnection bypasses replica-set discovery, which may advertise
	// member hostnames (e.g. host.docker.internal) that don't resolve locally.
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017/?directConnection=true"))
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("MongoDB not reachable: %v", err)
	}

	// Use a unique database name per test so runs don't interfere.
	dbName := "turbine_test_" + primitiveTimestamp()
	database := client.Database(dbName)

	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		_ = database.Drop(dropCtx)
		_ = client.Disconnect(dropCtx)
	})

	return New(database, lockQuantity, timeoutMinutes)
}

func primitiveTimestamp() string {
	// MongoDB database names cannot contain dots, so use a dot-free format.
	return time.Now().Format("20060102_150405_000000000")
}

func TestIntegration_SaveAndGetTasks(t *testing.T) {

	storage := testStorage(t, 16, 5)

	// Save a task that is ready to run (startDate in the past)
	task := queue.NewTask("hello", map[string]any{"key": "value"})
	task.StartDate = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, storage.SaveTask(task))

	// GetTasks locks and returns the ready task
	tasks, err := storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	require.Equal(t, "hello", tasks[0].Name)
	require.Equal(t, "value", tasks[0].Arguments["key"])
}

func TestIntegration_SaveTask_InvalidTaskID(t *testing.T) {

	storage := testStorage(t, 16, 5)

	task := queue.NewTask("x", nil)
	task.TaskID = "not-a-valid-objectid"

	require.Error(t, storage.SaveTask(task))
}

func TestIntegration_SaveTask_Update(t *testing.T) {

	storage := testStorage(t, 16, 5)

	// Save a new task and capture its generated ID
	task := queue.NewTask("first", nil)
	task.StartDate = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, storage.SaveTask(task))

	tasks, err := storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))

	// Update the existing task by re-saving with the same TaskID
	updated := tasks[0]
	updated.Name = "second"
	updated.LockID = "" // clear lock so it can be picked up again
	updated.TimeoutDate = 0
	updated.StartDate = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, storage.SaveTask(updated))

	tasks, err = storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks)) // still only one task (upsert, not insert)
	require.Equal(t, "second", tasks[0].Name)
}

func TestIntegration_DuplicateSignature(t *testing.T) {

	storage := testStorage(t, 16, 5)

	task := queue.NewTask("x", nil, queue.WithSignature("unique-sig"))
	require.NoError(t, storage.SaveTask(task))

	// Saving a second task with the same signature is silently dropped
	task2 := queue.NewTask("y", nil, queue.WithSignature("unique-sig"))
	require.NoError(t, storage.SaveTask(task2))

	count, err := storage.database.Collection(CollectionQueue).CountDocuments(context.Background(), bson.M{"signature": "unique-sig"})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestIntegration_DeleteTask(t *testing.T) {

	storage := testStorage(t, 16, 5)

	task := queue.NewTask("x", nil)
	task.StartDate = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, storage.SaveTask(task))

	tasks, err := storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))

	require.NoError(t, storage.DeleteTask(tasks[0].TaskID))

	count, err := storage.database.Collection(CollectionQueue).CountDocuments(context.Background(), bson.M{})
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func TestIntegration_DeleteTask_EmptyID(t *testing.T) {
	storage := testStorage(t, 16, 5)
	require.NoError(t, storage.DeleteTask(""))
}

func TestIntegration_DeleteTask_InvalidID(t *testing.T) {
	storage := testStorage(t, 16, 5)
	require.Error(t, storage.DeleteTask("not-a-valid-objectid"))
}

func TestIntegration_DeleteTaskBySignature(t *testing.T) {

	storage := testStorage(t, 16, 5)

	require.NoError(t, storage.SaveTask(queue.NewTask("x", nil, queue.WithSignature("sig-a"))))

	require.NoError(t, storage.DeleteTaskBySignature("sig-a"))

	count, err := storage.database.Collection(CollectionQueue).CountDocuments(context.Background(), bson.M{"signature": "sig-a"})
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func TestIntegration_DeleteTaskBySignature_Missing(t *testing.T) {
	storage := testStorage(t, 16, 5)
	// Deleting a signature that does not exist returns no error
	require.NoError(t, storage.DeleteTaskBySignature("does-not-exist"))
}

func TestIntegration_LogFailure(t *testing.T) {

	storage := testStorage(t, 16, 5)

	require.NoError(t, storage.LogFailure(queue.NewTask("failed-task", nil)))

	count, err := storage.database.Collection(CollectionLog).CountDocuments(context.Background(), bson.M{})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestIntegration_GetTasks_Empty(t *testing.T) {

	storage := testStorage(t, 16, 5)

	tasks, err := storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))
}

func TestIntegration_GetTasks_RespectsStartDate(t *testing.T) {

	storage := testStorage(t, 16, 5)

	// A task scheduled for the future should NOT be returned yet
	future := queue.NewTask("future", nil)
	future.StartDate = time.Now().Add(time.Hour).Unix()
	require.NoError(t, storage.SaveTask(future))

	tasks, err := storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))
}

// TestIntegration_SignatureRetryWriteback pins the retry path for tasks that carry a Signature
func TestIntegration_SignatureRetryWriteback(t *testing.T) {

	// queue.onTaskError bumps a task's counters and calls SaveTask to write them back.  When the
	// duplicate-signature check matched the task's OWN record, that write was dropped silently,
	// so RetryCount never advanced and the task retried forever.

	storage := testStorage(t, 16, 5)

	task := queue.NewTask("hello", nil, queue.WithSignature("sig-retry"))
	task.StartDate = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, storage.SaveTask(task))

	// Lock the task, the way the queue's poller does
	locked, err := storage.GetTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(locked))

	// Write the task back, as onTaskError does after a failed run
	retried := locked[0]
	retried.LockID = ""
	retried.TimeoutDate = 0
	retried.RetryCount = 5
	retried.Error = "boom"
	require.NoError(t, storage.SaveTask(retried))

	// The updated counters must be on disk, and still in a single record
	var stored queue.Task
	require.NoError(t, storage.database.Collection(CollectionQueue).
		FindOne(context.Background(), bson.M{"signature": "sig-retry"}).Decode(&stored))

	require.Equal(t, 5, stored.RetryCount, "retry counter must persist")
	require.Equal(t, int64(0), stored.TimeoutDate, "lock must be released")
	require.Equal(t, "boom", stored.Error)

	count, err := storage.database.Collection(CollectionQueue).
		CountDocuments(context.Background(), bson.M{"signature": "sig-retry"})
	require.NoError(t, err)
	require.Equal(t, int64(1), count, "write-back must update, not duplicate")
}

// TestIntegration_SignatureStillBlocksOtherTasks confirms the self-exclusion above did not
// weaken the dedup rule itself
func TestIntegration_SignatureStillBlocksOtherTasks(t *testing.T) {

	// A DIFFERENT task claiming a signature that is already queued is still dropped.

	storage := testStorage(t, 16, 5)

	require.NoError(t, storage.SaveTask(queue.NewTask("first", nil, queue.WithSignature("sig-dedup"))))

	// A second, independent task with the same signature is silently dropped
	require.NoError(t, storage.SaveTask(queue.NewTask("second", nil, queue.WithSignature("sig-dedup"))))

	var stored queue.Task
	require.NoError(t, storage.database.Collection(CollectionQueue).
		FindOne(context.Background(), bson.M{"signature": "sig-dedup"}).Decode(&stored))
	require.Equal(t, "first", stored.Name, "the original task must survive")

	count, err := storage.database.Collection(CollectionQueue).
		CountDocuments(context.Background(), bson.M{"signature": "sig-dedup"})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

// ignoringConsumer recognizes no Task, as a Consumer does after the task's name is retired
type ignoringConsumer struct{}

// OnPublish implements queue.Consumer
func (ignoringConsumer) OnPublish(*queue.Task) error { return nil }

// Run implements queue.Consumer, ignoring every Task
func (ignoringConsumer) Run(queue.Task) queue.Result { return queue.Ignored() }

// OnSuccess implements queue.Consumer
func (ignoringConsumer) OnSuccess(queue.Task) error { return nil }

// OnError implements queue.Consumer
func (ignoringConsumer) OnError(queue.Task, error) error { return nil }

// OnFailure implements queue.Consumer
func (ignoringConsumer) OnFailure(queue.Task, error) error { return nil }

// TestIntegration_UnrecognizedTaskFailsAtOnce confirms that a stored task no Consumer recognizes
// leaves the queue for the failure log on its first claim, instead of being locked and
// claimed again every time its lock expires (BUG-213)
func TestIntegration_UnrecognizedTaskFailsAtOnce(t *testing.T) {

	storage := testStorage(t, 16, 5)

	task := queue.NewTask("RetiredTaskName", map[string]any{"key": "value"})
	task.StartDate = time.Now().Add(-time.Minute).Unix()
	task.RetryMax = 8
	require.NoError(t, storage.SaveTask(task))

	q := queue.New(queue.WithStorage(storage), queue.WithPollStorage(true), queue.WithConsumers(ignoringConsumer{}))
	q.Start()
	t.Cleanup(q.Stop)

	countIn := func(collection string) int64 {
		count, err := storage.database.Collection(collection).CountDocuments(context.Background(), bson.M{"name": "RetiredTaskName"})
		require.NoError(t, err)
		return count
	}

	// The first poll claims the task.  Afterward it must be gone from the queue and in the failure log
	require.Eventually(t, func() bool { return countIn(CollectionQueue) == 0 }, 10*time.Second, 50*time.Millisecond, "the unrecognized task must leave the queue")
	require.Equal(t, int64(1), countIn(CollectionLog))
}
