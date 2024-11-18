//go:build localonly

package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/turbine/queue"
	"github.com/benpate/turbine/queue_mongo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestQueue_Immediate(t *testing.T) {

	// Logging Configuration
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	q := testQueue(t)
	require.NotNil(t, q)

	err := q.Publish(queue.NewTask("TestSuccess", nil))
	require.Nil(t, err)

	err = q.Publish(queue.NewTask("TestError", nil))
	require.Nil(t, err)

	err = q.Publish(queue.NewTask("TestFailure", nil))
	require.Nil(t, err)

	time.Sleep(8 * time.Second)
}

func TestQueue_Stored(t *testing.T) {

	// Logging Configuration
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	q := testQueue(t)
	require.NotNil(t, q)

	err := q.Publish(queue.NewTask("TestSuccess", nil, queue.WithPriority(100)))
	require.Nil(t, err)

	err = q.Publish(queue.NewTask("TestError", nil, queue.WithPriority(110)))
	require.Nil(t, err)

	err = q.Publish(queue.NewTask("TestFailure", nil, queue.WithPriority(120)))
	require.Nil(t, err)

	time.Sleep(8 * time.Second)
}

// testQueue creates a new queue+storage on the local mongodb server.
func testQueue(t *testing.T) queue.Queue {

	// Create a new MongoDB client
	mongoOptions := options.ClientOptions{}
	mongoOptions.ApplyURI("mongodb://localhost:27017")
	mongoClient, err := mongo.Connect(context.Background(), &mongoOptions)
	require.Nil(t, err)

	mongoDatabase := mongoClient.Database("TestTurbine")
	mongoStorage := queue_mongo.New(mongoDatabase, 1, 1)

	return queue.New(
		queue.WithStorage(mongoStorage),
		queue.WithConsumers(testConsumer),
		queue.WithRunImmediatePriority(10),
	)
}

// testConsumer reports success, error, and failure for testing purposes.
func testConsumer(name string, _ map[string]any) queue.Result {

	log.Trace().Str("name", name).Msg("testConsumer. Running task:")

	switch name {

	case "TestSuccess":
		return queue.Success()

	case "TestError":
		return queue.Error(derp.NewInternalError("TestError", "This is a test error"))

	case "TestFailure":
		return queue.Failure(derp.NewInternalError("TestFailure", "This is a test failure"))
	}

	return queue.Ignored()
}
