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
	"io"
)

type Transformation struct {
	From Nature `json:"from"`
	To   Nature `json:"to"`
}

type Nature struct {
	EncodingID EncodingID `json:"encodingID"`
	Dialect    string     `json:"dialect"`
}

// DecodeText the stream and encodes it to UTF8
// We close the io.ReadCloser
func (t Transformation) DecodeText(r io.ReadCloser) (UTF8String, error) {
	defer ignoreErr(r.Close())
	s, err := ReaderToUTF8(r, t.From.EncodingID)
	if err != nil {
		return "", err
	}
	return s, nil
}

// EncodeResult ethe transformation result to the Transformation destination encoding.
// We close the io.WriteCloser
func (t Transformation) EncodeResult(res Result, w io.WriteCloser) error {
	defer ignoreErr(w.Close())
	text := res.Render() // render the result as a Text
	return FromUTF8ToWriter(text, t.To.EncodingID, w)
}
