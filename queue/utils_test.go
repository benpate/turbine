package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackoff(t *testing.T) {

	test := func(retryCount int, expected int) {
		require.Equal(t, time.Duration(expected)*time.Minute, backoff(retryCount))
	}

	test(0, 1)
	test(1, 2)
	test(2, 4)
	test(3, 8)
	test(4, 16)
	test(5, 32)
	test(6, 64)
	test(7, 128)
	test(8, 256)
	test(9, 512)
	test(10, 1024)
	test(11, 2048)
	test(12, 4096)
}
