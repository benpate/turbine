package queue

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// spoolStorage is a Storage that hands each saved Task to the next GetTasks call, as a real
// provider does, and counts the polls it receives
type spoolStorage struct {
	mockStorage
	mutex   sync.Mutex
	pending []Task
	polls   int
	pollErr error
}

// SaveTask holds a Task for the next poll
func (storage *spoolStorage) SaveTask(task Task) error {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	storage.pending = append(storage.pending, task)
	return nil
}

// GetTasks returns every Task saved since the last poll
func (storage *spoolStorage) GetTasks() ([]Task, error) {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	storage.polls++

	if storage.pollErr != nil {
		return nil, storage.pollErr
	}

	result := storage.pending
	storage.pending = nil
	return result, nil
}

// DeleteTask accepts every delete, because a finished Task is already off the spool
func (storage *spoolStorage) DeleteTask(string) error {
	return nil
}

// LogFailure records a failed Task under the lock, because several workers may fail at once
func (storage *spoolStorage) LogFailure(task Task) error {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	storage.failures = append(storage.failures, task)
	return nil
}

// pollCount returns the number of polls received so far
func (storage *spoolStorage) pollCount() int {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	return storage.polls
}

// requirePolls waits until the storage has been polled at least `count` times
func requirePolls(t *testing.T, storage *spoolStorage, count int) {
	t.Helper()

	require.Eventually(t, func() bool {
		return storage.pollCount() >= count
	}, 2*time.Second, time.Millisecond, "expected %d polls", count)
}

// startIdleQueue starts a Queue whose poller has found nothing and is waiting out a poll
// interval far longer than the test
func startIdleQueue(t *testing.T, storage *spoolStorage, consumers ...consumerFunc) *Queue {
	t.Helper()

	q := New(WithStorage(storage), withConsumerFuncs(consumers...))
	q.pollInterval = time.Hour
	q.Start()
	t.Cleanup(q.Stop)

	requirePolls(t, storage, 1)
	return q
}

func TestPublish_SignedTaskWakesIdlePoller(t *testing.T) {

	ran := make(chan Task, 1)
	storage := &spoolStorage{}

	q := startIdleQueue(t, storage, func(task Task) Result {
		ran <- task
		return Success()
	})

	// A signed Task cannot run from memory, so it reaches the consumer only through a poll
	require.NoError(t, q.Publish(NewTask("signed", nil, WithSignature("one"))))

	select {
	case task := <-ran:
		require.Equal(t, "signed", task.Name)
		require.Equal(t, "one", task.Signature)
	case <-time.After(time.Second):
		t.Fatal("a signed task waited for the poll interval")
	}
}

func TestPublish_FutureTaskDoesNotWakePoller(t *testing.T) {

	storage := &spoolStorage{}
	q := startIdleQueue(t, storage)

	require.NoError(t, q.Publish(NewTask("later", nil, WithDelaySeconds(3600))))

	require.Empty(t, q.wake, "a task that is not due must not ask for a poll")
	require.Never(t, func() bool {
		return storage.pollCount() > 1
	}, 50*time.Millisecond, 5*time.Millisecond)
}

func TestSchedule_WakesPollerOnlyWhenDue(t *testing.T) {

	storage := &spoolStorage{}
	q := startIdleQueue(t, storage)

	// A delayed Task leaves the poller alone
	require.NoError(t, q.Schedule(NewTask("later", nil), time.Hour))
	require.Empty(t, q.wake)

	// A Task scheduled with no delay is due now
	require.NoError(t, q.Schedule(NewTask("now", nil), 0))
	requirePolls(t, storage, 2)
}

func TestPublish_BurstCostsOnePoll(t *testing.T) {

	// No poller runs here, so every request stays where kick left it
	q := New(WithStorage(&spoolStorage{}))

	for index := 0; index < 100; index++ {
		require.NoError(t, q.Publish(NewTask("burst", nil, WithSignature(string(rune('a'+index))))))
	}

	require.Len(t, q.wake, 1, "the wake channel holds a single request")
}

func TestPublish_ImmediateTaskDoesNotWakePoller(t *testing.T) {

	// An unsigned Task at the default priority goes straight to the buffer
	q := New(WithStorage(&spoolStorage{}))

	require.NoError(t, q.Publish(NewTask("immediate", nil)))
	require.Len(t, q.buffer, 1)
	require.Empty(t, q.wake)
}

func TestWaitForWork_PollInterval(t *testing.T) {

	q := New()
	q.pollInterval = 10 * time.Millisecond

	started := time.Now()
	require.True(t, q.waitForWork())
	require.GreaterOrEqual(t, time.Since(started), 10*time.Millisecond)
}

func TestWaitForWork_PendingKickReturnsAtOnce(t *testing.T) {

	q := New()
	q.pollInterval = time.Hour

	// A kick that arrives while the poller is busy is kept for its next wait
	q.kick(NewTask("due", nil))

	finished := make(chan bool, 1)
	go func() { finished <- q.waitForWork() }()

	select {
	case result := <-finished:
		require.True(t, result)
	case <-time.After(time.Second):
		t.Fatal("a pending kick did not end the wait")
	}

	require.Empty(t, q.wake, "the wait consumes the request")
}

func TestWaitForWork_Stop(t *testing.T) {

	q := New()
	q.pollInterval = time.Hour

	finished := make(chan bool, 1)
	go func() { finished <- q.waitForWork() }()

	q.Stop()

	select {
	case result := <-finished:
		require.False(t, result)
	case <-time.After(time.Second):
		t.Fatal("Stop did not end the wait")
	}
}

func TestStart_StopEndsIdlePoller(t *testing.T) {

	storage := &spoolStorage{}
	q := New(WithStorage(storage))
	q.pollInterval = time.Hour

	finished := make(chan struct{})
	go func() {
		q.start()
		close(finished)
	}()

	requirePolls(t, storage, 1)
	q.Stop()

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("the poller outlived Stop")
	}
}

func TestStart_StorageErrorWaitsThenRetries(t *testing.T) {

	storage := &spoolStorage{pollErr: errors.New("storage is down")}
	q := New(WithStorage(storage))
	q.pollInterval = time.Hour

	finished := make(chan struct{})
	go func() {
		q.start()
		close(finished)
	}()

	// The failed poll is followed by a wait, not by a second poll
	requirePolls(t, storage, 1)
	require.Never(t, func() bool {
		return storage.pollCount() > 1
	}, 50*time.Millisecond, 5*time.Millisecond)

	// A kick ends that wait early, and the poller tries again
	q.kick(NewTask("due", nil))
	requirePolls(t, storage, 2)

	q.Stop()

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("the poller outlived Stop")
	}
}
