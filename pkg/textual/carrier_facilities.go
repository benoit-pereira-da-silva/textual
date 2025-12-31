package textual

import (
	"encoding/json"
	"errors"
	"fmt"
)

func StringFrom(from UTF8String) StringCarrier {
	return (*new(StringCarrier)).FromUTF8String(from)
}

func JSONCarrierFrom[T any](from UTF8String) JsonGenericCarrier[T] {
	return (*new(JsonGenericCarrier[T])).FromUTF8String(from)
}

func JSONFrom(from UTF8String) JsonCarrier {
	return (*new(JsonCarrier)).FromUTF8String(from)
}

func ParcelFrom(from UTF8String) Parcel {
	return (*new(Parcel)).FromUTF8String(from)
}

// CastJson attempts to convert a JsonCarrier carrier's Value into a specified type T.
// Returns the cast value or an error if the casting/unmarshaling fails.
func CastJson[T any](j JsonCarrier) (T, error) {
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
			return out, fmt.Errorf("json carrier (index %d): %w", j.Index, errors.New("empty JsonCarrier value"))
		}
		return out, errors.New("empty JsonCarrier value")
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
	if jPtr, ok := any(&out).(*JsonCarrier); ok {
		*jPtr = j
		jPtr.Error = nil
		return out, nil
	}
	// General case: cast JsonCarrier.Value into T by unmarshaling.
	if err := json.Unmarshal(j.Value, &out); err != nil {
		if j.Index != 0 {
			return out, fmt.Errorf("json unmarshal (index %d): %w", j.Index, err)
		}
		return out, err
	}
	return out, nil
}
