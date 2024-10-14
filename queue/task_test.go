package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTask(t *testing.T) {

	task := NewTask("Hello World", map[string]interface{}{"key": "value"})

	require.Equal(t, "Hello World", task.Name)
	require.Equal(t, "value", task.Arguments["key"])
}
