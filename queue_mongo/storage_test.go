package queue_mongo

import (
	"testing"

	"github.com/benpate/turbine/queue"
)

func TestStorage(t *testing.T) {

	var _ queue.Storage = Storage{}
}
