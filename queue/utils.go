package queue

import (
	"time"
)

// maxBackoffExponent caps the backoff shift so that it cannot overflow time.Duration.
const maxBackoffExponent = 27

// backoff calculates the exponential backoff time for a retry
func backoff(retryCount int) time.Duration {

	// RULE: The shift count is clamped at BOTH ends.  Above maxBackoffExponent it would overflow
	// time.Duration (see above); below zero -- a corrupt or hand-edited record -- a variable shift
	// by a negative amount panics at run time, so a nonsense count is treated as the first attempt.
	exponent := min(max(retryCount, 0), maxBackoffExponent)

	return time.Duration(1<<exponent) * time.Minute
}
