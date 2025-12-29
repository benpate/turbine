package queue_filesystem

import (
	"testing"

	"github.com/benpate/turbine/queue"
)

func TestStorage(_ *testing.T) {

	var _ queue.Storage = Storage{}
}
