package carrier

import (
	"encoding/json"
	"errors"
	"fmt"
)

// CastJSON attempts to convert a JSON carrier's Value into a specified type T.
// Returns the cast value or an error if the casting/unmarshaling fails.
func CastJSON[T any](j JSON) (T, error) {
	var out T
	// If an upstream stage already produced an error, do NOT attempt any casting.
	if j.Error != nil {
		if j.Index != 0 {
			return out, fmt.Errorf("json carrier (index %d): %w", j.Index, j.Error)
		}
		return out, j.Error
	}
	if len(j.Value) == 0 {
		if j.Index != 0 {
			return out, fmt.Errorf("json carrier (index %d): %w", j.Index, errors.New("empty JSON value"))
		}
		return out, errors.New("empty JSON value")
	}
	// Fast paths for “raw” targets.
	if rmPtr, ok := any(&out).(*json.RawMessage); ok {
		*rmPtr = append(json.RawMessage(nil), j.Value...)
		return out, nil
	}
	if bPtr, ok := any(&out).(*[]byte); ok {
		*bPtr = append([]byte(nil), j.Value...)
		return out, nil
	}
	if jPtr, ok := any(&out).(*JSON); ok {
		*jPtr = j
		jPtr.Error = nil
		return out, nil
	}
	// General case: cast JSON.Value into T by unmarshaling.
	if err := json.Unmarshal(j.Value, &out); err != nil {
		if j.Index != 0 {
			return out, fmt.Errorf("json unmarshal (index %d): %w", j.Index, err)
		}
		return out, err
	}
	return out, nil
}
