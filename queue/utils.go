package queue

import (
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

	// RULE: The shift count is clamped at BOTH ends.  Above maxBackoffExponent it would overflow
	// time.Duration (see above); below zero -- a corrupt or hand-edited record -- a variable shift
	// by a negative amount panics at run time, so a nonsense count is treated as the first attempt.
	exponent := min(max(retryCount, 0), maxBackoffExponent)

	return time.Duration(1<<exponent) * time.Minute
}
