package queue_filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDeleteTask_InvalidID pins the ObjectID guard: TaskIDs that aren't valid
// ObjectID hex are rejected before any filesystem path is built, so a
// malformed ID can never reach outside the storage directory.
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
