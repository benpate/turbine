package queue

import (
	"math"
	"time"
)

// maxBackoffExponent caps the retry count used in the backoff calculation to
// prevent an integer overflow. time.Duration is an int64 of nanoseconds (max
// ~292 years), so 2^28 minutes already overflows it and wraps to a negative
// duration -- which would schedule the retry in the past and hot-loop the queue.
// 27 is the largest safe exponent: backoff(27) = 2^27 minutes ≈ 255 years, far
// longer than any real retry policy, so this clamps nothing in practice.
const maxBackoffExponent = 27

// backoff calculates the exponential backoff time for a retry
func backoff(retryCount int) time.Duration {

	// Clamp the exponent to avoid overflowing time.Duration (see maxBackoffExponent)
	if retryCount > maxBackoffExponent {
		retryCount = maxBackoffExponent
	}

	return time.Duration(math.Pow(2, float64(retryCount))) * time.Minute
}
