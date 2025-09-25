package queue_filesystem

import (
	"testing"

	"github.com/benpate/turbine/queue"
)

func TestStorage(t *testing.T) {

	var _ queue.Storage = Storage{}
}
