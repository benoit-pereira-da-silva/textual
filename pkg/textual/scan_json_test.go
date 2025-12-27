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
	"bufio"
	"strings"
	"testing"
)

func TestScanJSON_SplitsObjectsAndArrays(t *testing.T) {
	input := " \n,\t{\"a\":1}  [1,2,{\"b\":\"x\"}]  {\"c\":\"{[\\\"}]\"}\n"

	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(ScanJSON)

	var tokens []string
	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	want := []string{
		`{"a":1}`,
		`[1,2,{"b":"x"}]`,
		`{"c":"{[\"}]"}`,
	}
	if len(tokens) != len(want) {
		t.Fatalf("unexpected token count: got %d want %d tokens=%#v", len(tokens), len(want), tokens)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Fatalf("token %d mismatch: got %q want %q", i, tokens[i], want[i])
		}
	}
}

func TestScanJSON_UnexpectedEOF(t *testing.T) {
	input := `{"a": [1, 2, 3}`

	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(ScanJSON)

	// Scan should fail because EOF happens before closing the array/object.
	for scanner.Scan() {
	}

	if err := scanner.Err(); err == nil {
		t.Fatalf("expected scanner error, got nil")
	}
}
