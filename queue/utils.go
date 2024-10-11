package queue

import (
	"math"
	"time"
)

// backoff calculates the exponential backoff time for a retry
func backoff(retryCount int) time.Duration {
	return time.Duration(math.Pow(2, float64(retryCount))) * time.Minute
}
