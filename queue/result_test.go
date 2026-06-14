package queue

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResult_Success(t *testing.T) {
	result := Success()
	require.Equal(t, ResultStatusSuccess, result.Status)
	require.True(t, result.IsSuccessful())
	require.False(t, result.NotSuccessful())
}

func TestResult_Requeue(t *testing.T) {
	result := Requeue(5 * time.Minute)
	require.Equal(t, ResultStatusRequeue, result.Status)
	require.Equal(t, 5*time.Minute, result.Delay)
	require.True(t, result.IsSuccessful())
	require.False(t, result.NotSuccessful())
}

func TestResult_Ignored(t *testing.T) {
	result := Ignored()
	require.Equal(t, ResultStatusIgnored, result.Status)
	require.False(t, result.IsSuccessful())
	require.True(t, result.NotSuccessful())
}

func TestResult_Error(t *testing.T) {
	err := errors.New("boom")
	result := Error(err)
	require.Equal(t, ResultStatusError, result.Status)
	require.Equal(t, err, result.Error)
	require.False(t, result.IsSuccessful())
	require.True(t, result.NotSuccessful())
}

func TestResult_Failure(t *testing.T) {
	err := errors.New("fatal")
	result := Failure(err)
	require.Equal(t, ResultStatusFailure, result.Status)
	require.Equal(t, err, result.Error)
	require.False(t, result.IsSuccessful())
	require.True(t, result.NotSuccessful())
}
