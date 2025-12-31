// Copyright 2026 Benoit Pereira da Silva
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package textual

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

func StringFrom(s UTF8String) StringCarrier {
	return (*new(StringCarrier)).FromUTF8String(s)
}

func JSONCarrierFrom[T any](s UTF8String) JsonGenericCarrier[T] {
	return (*new(JsonGenericCarrier[T])).FromUTF8String(s)
}

func JSONFrom(s UTF8String) JsonCarrier {
	return (*new(JsonCarrier)).FromUTF8String(s)
}

func CSVFrom(s UTF8String) CsvCarrier {
	return (*new(CsvCarrier)).FromUTF8String(s)
}

func XMLFrom(s UTF8String) XmlCarrier {
	return (*new(XmlCarrier)).FromUTF8String(s)
}

func ParcelFrom(s UTF8String) Parcel {
	return (*new(Parcel)).FromUTF8String(s)
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

// CastCsvRecord attempts to parse a CsvCarrier.Value into a single CSV record ([]string)
// using Go's standard library (encoding/csv).
//
// The carrier is expected to contain one record (as produced by ScanCSV).
// If it contains multiple records, CastCsvRecord returns an error.
//
// If the carrier carries an upstream error, it is returned and parsing is skipped.
func CastCsvRecord(c CsvCarrier) ([]string, error) {
	// If an upstream stage already produced an error, do NOT attempt parsing.
	if c.Error != nil {
		if c.Index != 0 {
			return nil, fmt.Errorf("csv carrier (index %d): %w", c.Index, c.Error)
		}
		return nil, c.Error
	}
	if len(c.Value) == 0 {
		if c.Index != 0 {
			return nil, fmt.Errorf("csv carrier (index %d): %w", c.Index, errors.New("empty CsvCarrier value"))
		}
		return nil, errors.New("empty CsvCarrier value")
	}

	r := csv.NewReader(strings.NewReader(string(c.Value)))
	// A single-record parse should not enforce field count consistency across records.
	r.FieldsPerRecord = -1

	rec, err := r.Read()
	if err != nil {
		if c.Index != 0 {
			return nil, fmt.Errorf("csv read (index %d): %w", c.Index, err)
		}
		return nil, err
	}

	// Ensure there isn't a second record encoded in the same token.
	_, err2 := r.Read()
	if err2 != io.EOF {
		if err2 == nil {
			err2 = errors.New("multiple CSV records found in one CsvCarrier value")
		}
		if c.Index != 0 {
			return nil, fmt.Errorf("csv read (index %d): %w", c.Index, err2)
		}
		return nil, err2
	}

	return rec, nil
}

// CastXml attempts to convert an XmlCarrier's Value into a specified type T.
// Returns the cast value or an error if unmarshaling fails.
//
// If the carrier carries an upstream error, it is returned and unmarshaling is skipped.
func CastXml[T any](x XmlCarrier) (T, error) {
	var out T
	// If an upstream stage already produced an error, do NOT attempt parsing.
	if x.Error != nil {
		if x.Index != 0 {
			return out, fmt.Errorf("xml carrier (index %d): %w", x.Index, x.Error)
		}
		return out, x.Error
	}
	if len(x.Value) == 0 {
		if x.Index != 0 {
			return out, fmt.Errorf("xml carrier (index %d): %w", x.Index, errors.New("empty XmlCarrier value"))
		}
		return out, errors.New("empty XmlCarrier value")
	}

	// Fast paths for “raw” targets.
	if bPtr, ok := any(&out).(*[]byte); ok {
		*bPtr = append([]byte(nil), []byte(x.Value)...)
		return out, nil
	}
	if sPtr, ok := any(&out).(*string); ok {
		*sPtr = string(x.Value)
		return out, nil
	}
	if xPtr, ok := any(&out).(*XmlCarrier); ok {
		*xPtr = x
		xPtr.Error = nil
		return out, nil
	}

	// General case: unmarshal XML fragment into T.
	if err := xml.Unmarshal([]byte(x.Value), &out); err != nil {
		if x.Index != 0 {
			return out, fmt.Errorf("xml unmarshal (index %d): %w", x.Index, err)
		}
		return out, err
	}
	return out, nil
}
