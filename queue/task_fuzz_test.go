package queue

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/require"
)

// FuzzTaskJSONRoundTrip confirms that any Task survives a JSON marshal/unmarshal
// cycle unchanged. The filesystem storage provider persists tasks as JSON, so a
// task pulled back off disk must be identical to the one that was saved.
//
// The fuzzer drives the four free-text string fields (the realistic source of
// serialization surprises: quotes, control bytes, unicode) plus one numeric and
// one timestamp field. The remaining numeric fields are pinned to representative
// values around them, keeping the closure within a sane parameter count while
// still asserting whole-struct equality.
func FuzzTaskJSONRoundTrip(f *testing.F) {

	// Seed with representative field values: typical, empty, and negative cases.
	f.Add("taskId", "name", "signature", "errorText", 16, int64(200))
	f.Add("", "", "", "", 0, int64(0))
	f.Add("id", "name", "", "", -1, int64(-1))

	f.Fuzz(func(t *testing.T, taskID, name, signature, errorText string, priority int, startDate int64) {

		// Skip inputs containing invalid UTF-8. encoding/json replaces invalid
		// byte sequences with U+FFFD on marshal, so the round trip is only exact
		// for valid UTF-8 -- which is what every realistic task field contains.
		if !utf8.ValidString(taskID) || !utf8.ValidString(name) || !utf8.ValidString(signature) || !utf8.ValidString(errorText) {
			t.Skip()
		}

		// Build a Task from the fuzzed inputs. Arguments are exercised separately
		// in FuzzTaskArgumentsRoundTrip, so keep them nil here.
		original := Task{
			TaskID:      taskID,
			LockID:      taskID, // reuse the fuzzed string to cover LockID too
			Name:        name,
			Signature:   signature,
			Error:       errorText,
			CreateDate:  startDate,
			StartDate:   startDate,
			TimeoutDate: startDate,
			Priority:    priority,
			RetryCount:  priority,
			RetryMax:    priority,
		}

		// Marshal to JSON, then back into a fresh Task
		data, err := json.Marshal(original)
		require.NoError(t, err)

		decoded := Task{}
		require.NoError(t, json.Unmarshal(data, &decoded))

		// AsyncDelay is excluded from serialization (`bson:"-"` / not persisted),
		// so it must remain at its zero value after a round trip.
		require.Equal(t, 0, decoded.AsyncDelay)

		// Every persisted field must survive the round trip exactly.
		require.Equal(t, original, decoded)
	})
}

// FuzzTaskArgumentsRoundTrip confirms that a Task's Arguments map survives a JSON
// round trip. Arguments are caller-supplied data of arbitrary shape, so they are
// the most likely field to drift during serialization.
func FuzzTaskArgumentsRoundTrip(f *testing.F) {

	f.Add("key", "value")
	f.Add("", "")
	f.Add("nested.key", "🜂 unicode ⚙")

	f.Fuzz(func(t *testing.T, key, value string) {

		// encoding/json only preserves valid UTF-8 (see FuzzTaskJSONRoundTrip),
		// so skip inputs that the standard library is documented not to round trip.
		if !utf8.ValidString(key) || !utf8.ValidString(value) {
			t.Skip()
		}

		original := NewTask("fuzz", mapof.Any{key: value})

		// Marshal to JSON, then back into a fresh Task
		data, err := json.Marshal(original)
		require.NoError(t, err)

		decoded := Task{}
		require.NoError(t, json.Unmarshal(data, &decoded))

		// The single argument must come back with the same key and value.
		require.Equal(t, value, decoded.Arguments.GetString(key))
	})
}
