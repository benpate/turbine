package queue

// mockStorage is an in-memory Storage implementation for testing.  It records every call, and
// its error fields let a test exercise the failure paths.
type mockStorage struct {
	tasks        []Task
	saved        []Task
	deleted      []string
	deletedBySig []string
	failures     []Task

	getTasksErr  error
	saveErr      error
	deleteErr    error
	deleteSigErr error
	logErr       error
}

func (m *mockStorage) GetTasks() ([]Task, error) {
	return m.tasks, m.getTasksErr
}

func (m *mockStorage) SaveTask(task Task) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = append(m.saved, task)
	return nil
}

func (m *mockStorage) DeleteTask(taskID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleted = append(m.deleted, taskID)
	return nil
}

func (m *mockStorage) DeleteTaskBySignature(signature string) error {
	if m.deleteSigErr != nil {
		return m.deleteSigErr
	}
	m.deletedBySig = append(m.deletedBySig, signature)
	return nil
}

func (m *mockStorage) LogFailure(task Task) error {
	if m.logErr != nil {
		return m.logErr
	}
	m.failures = append(m.failures, task)
	return nil
}
