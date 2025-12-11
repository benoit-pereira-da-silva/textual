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
	"reflect"
	"strings"
	"testing"
)

// TestScanExpression_ReconstructsInput ensures that concatenating all tokens
// produced by ScanExpression yields the original input.
func TestScanExpression_ReconstructsInput(t *testing.T) {
	const input = "Hello, world!\nThis  is\ttextual.\n"

	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(ScanExpression)

	var out strings.Builder
	for scanner.Scan() {
		out.Write(scanner.Bytes())
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	if got := out.String(); got != input {
		t.Fatalf("reconstructed text mismatch:\n got: %q\nwant: %q", got, input)
	}
}

// TestScanExpression_TokensIncludeSpacesAndNewlines verifies that punctuation,
// spaces and line breaks are attached to the surrounding words.
func TestScanExpression_TokensIncludeSpacesAndNewlines(t *testing.T) {
	const input = "Hello, world!\n"

	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(ScanExpression)

	var tokens []string
	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	want := []string{"Hello, ", "world!\n"}
	if !reflect.DeepEqual(tokens, want) {
		t.Fatalf("unexpected tokens:\n got: %#v\nwant: %#v", tokens, want)
	}
}
