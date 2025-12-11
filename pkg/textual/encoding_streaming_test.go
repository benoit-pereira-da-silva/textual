// Copyright 2026 Benoit Pereira da Silva
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package textual

import (
	"bytes"
	"io"
	"testing"
)

// TestNewUTF8Reader_ISO8859_1 checks that NewUTF8Reader decodes ISO‑8859‑1
// into UTF‑8 as a streaming reader.
func TestNewUTF8Reader_ISO8859_1(t *testing.T) {
	// "Café" encoded as ISO‑8859‑1: 43 61 66 E9
	encoded := []byte{0x43, 0x61, 0x66, 0xE9}

	r := bytes.NewReader(encoded)
	utf8Reader, err := NewUTF8Reader(r, ISO8859_1)
	if err != nil {
		t.Fatalf("NewUTF8Reader returned error: %v", err)
	}

	out, err := io.ReadAll(utf8Reader)
	if err != nil {
		t.Fatalf("ReadAll on UTF‑8 reader failed: %v", err)
	}

	if string(out) != "Café" {
		t.Fatalf("unexpected decoded string: got %q, want %q", string(out), "Café")
	}
}
