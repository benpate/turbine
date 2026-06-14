package queue_mongo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// NOTE: The Storage methods (SaveTask, GetTasks, DeleteTask, LogFailure, etc.)
// all require a live MongoDB connection and are therefore covered by integration
// tests rather than unit tests. The cases below exercise only the parts of the
// package that do not touch the database.

func TestNew(t *testing.T) {

	// New accepts a nil database for construction; the field assignments are
	// what we verify here.
	storage := New(nil, 32, 5)

	require.Nil(t, storage.database)
	require.Equal(t, 32, storage.lockQuantity)
	require.Equal(t, 5, storage.timeoutMinutes)
}

func TestConstants(t *testing.T) {
	require.Equal(t, "Queue", CollectionQueue)
	require.Equal(t, "QueueErrors", CollectionLog)
}
