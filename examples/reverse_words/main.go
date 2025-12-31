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

package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"time"
	"unicode"

	textual "github.com/benoit-pereira-da-silva/textual/pkg/textual"
)

//go:embed files
var FS embed.FS

// defaultExcerptPath is the relative path of the Baudelaire excerpt used by the
// sample. It is expected to live next to main.go.
const defaultExcerptPath = "files/les_fleurs_du_mal.txt"

var (
	minDelayMS = 1  // minimum delay between batches in milliseconds
	maxDelayMS = 50 // maximum delay between batches in milliseconds
)

// caseKind models the original casing of a rune at a given position inside a
// word. It allows the processor to restore a similar casing pattern once the
// characters have been reversed.
type caseKind int

const (
	caseOther caseKind = iota // non-letter or letter with no specific casing
	caseUpper                 // uppercase letter in the original word
	caseLower                 // lowercase letter in the original word
)

// main wires the reverse-words processor into an IOReaderProcessor that streams
// text from disk or embedded fs, reverses every word,
// waits a small random delay between each batch, and prints the transformed
// tokens to stdout.
//
// Usage:
//
//	go run main.go
//	go run main.go --twice
//
// When --twice is provided, the reverse-words processor is chained twice.
func main() {
	// Command-line flags.
	twice := flag.Bool("twice", false, "apply the reverse-words processor twice")
	inputPath := flag.String("input", defaultExcerptPath, "path to the input text file (UTF-8)")
	minDelay := flag.Int("min-delay", minDelayMS, "minimum delay in milliseconds before processing a token")
	maxDelay := flag.Int("max-delay", maxDelayMS, "maximum delay between processed tokens in milliseconds")
	wordByWord := flag.Bool("word-by-word", false, "use word-by-word tokenization (ScanExpression)")
	flag.Parse()

	var f fs.File
	var err error

	if *inputPath != defaultExcerptPath {
		// Use the os FS.
		f, err = os.Open(*inputPath)
	} else {
		// Use the embed FS.
		f, err = FS.Open(defaultExcerptPath)
	}
	minDelayMS = *minDelay
	maxDelayMS = *maxDelay
	if maxDelayMS < minDelayMS {
		maxDelayMS = minDelayMS + 1
	}

	if err != nil {
		log.Fatalf("unable to open input file %q: %v", *inputPath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			log.Printf("warning: failed to close input file: %v", cerr)
		}
	}()

	// Build the reverse-words processor; optionally chain it twice.
	// We use a textual.StringCarrier because a textual.Parcel is useless in this context.
	// buildProcessor[textual.Parcel](*twice) works too.!
	processor := buildProcessor[textual.StringCarrier](*twice)

	// Construct an IOReaderProcessor that will scan the file token-by-token and
	// feed each token as a textual.StringCarrier into the processor.
	ioProc := textual.NewIOReaderProcessor(processor, f)
	if *wordByWord {
		// Switch from line-based tokenization to expression tokenization:
		// [optional leading whitespace][word/punctuation][optional trailing whitespace/newlines]
		ioProc.SetSplitFunc(textual.ScanExpression)
	}

	// Use a background context for this small example. In a real application
	// you would probably derive it from a parent context or hook it to signals.
	ioProc.SetContext(context.Background())

	// Start the streaming pipeline.
	out := ioProc.Start()

	for res := range out {
		// Render the textual.Carrier back to a UTF-8 string and display it on stdout.
		str := res.UTF8String()
		if *wordByWord {
			fmt.Print(str)
		} else {
			fmt.Println(str)
		}
	}
}

// buildProcessor constructs the processing pipeline.
//
// If twice is false, the returned Processor is a single reverse-words stage.
// If twice is true, two reverse-words processors are chained with textual.Chain
// so that words are reversed twice in a row (resulting in the original text).
func buildProcessor[S textual.Carrier[S]](twice bool) textual.Processor[S] {

	// makeReverseStage returns a single reverse-words stage.
	//
	// Each stage gets its own random source so that chaining the stage twice does
	// not introduce shared mutable state between goroutines.
	makeReverseStage := func() textual.Processor[S] {
		// Seed a local random source used to add a small delay between each batch
		// of transformed text.
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

		return textual.ProcessorFunc[S](func(ctx context.Context, in <-chan S) <-chan S {
			return textual.Async(ctx, in, func(ctx context.Context, res S) S {
				// Transform the token by reversing each word while keeping
				// punctuation and whitespace in place.
				transformed := reverseWords(res.UTF8String())

				// Wait for a random delay between min-delay and max-delay
				// milliseconds to simulate a streaming / progressive workload.
				//
				// The wait is context-aware so a canceled pipeline does not
				// keep sleeping needlessly.
				delayRange := maxDelayMS - minDelayMS + 1
				delay := minDelayMS + rnd.Intn(delayRange)

				timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
				defer timer.Stop()

				select {
				case <-ctx.Done():
					// Cancellation is out-of-band: the stage will stop emitting soon.
					// Returning any value is fine; Async will not send once ctx is done.
					return res
				case <-timer.C:
					// Proceed.
				}

				// Create a new carrier, preserving the index (ordering hint) and
				// any existing per-item error.
				return res.FromUTF8String(transformed).
					WithIndex(res.GetIndex()).
					WithError(res.GetError())
			})
		})
	}

	if !twice {
		return makeReverseStage()
	}

	// Chain the reverse-words processor twice. Each token is processed by the
	// first stage, then by the second stage. Applying the same transformation
	// twice restores the original text (modulo casing rules).
	return textual.NewChain(makeReverseStage(), makeReverseStage())
}

/////////////////////
//  reverse words
/////////////////////

// reverseWords applies a word-wise reversal on the given UTF-8 string while
// preserving punctuation, whitespace and the original casing pattern.
//
// Behaviour:
//
//   - "Words" are defined as contiguous sequences of Unicode letters or digits.
//     Everything else (spaces, punctuation, symbols, …) is left untouched and
//     stays at the same position.
//   - For each word, the runes are reversed.
//   - The casing pattern of the original word is preserved by position: if the
//     rune at index 0 was uppercase in the original word, the rune that ends up
//     at index 0 after reversal is also uppercased, and so on.
//   - Non-letter runes within a word (digits, etc.) keep their original form.
//
// Example:
//
//	"Ciel,"   -> "Leic,"
//	"Bonjour" -> "Ruojnob"
//	"WORLD!"  -> "DLROW!"
func reverseWords(input textual.UTF8String) textual.UTF8String {
	runes := []rune(string(input))
	n := len(runes)

	// Helper to determine if a rune belongs to a "word".
	isWordRune := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	}

	// reverseSegment reverses the runes in [start,end) and reapplies a casing
	// pattern based on the original runes at those positions.
	reverseSegment := func(start, end int) {
		length := end - start
		if length <= 1 {
			return
		}

		// Snapshot original runes and their casing.
		letters := make([]rune, length)
		cases := make([]caseKind, length)

		for i := 0; i < length; i++ {
			r := runes[start+i]
			letters[i] = r

			switch {
			case unicode.IsUpper(r):
				cases[i] = caseUpper
			case unicode.IsLower(r):
				cases[i] = caseLower
			default:
				cases[i] = caseOther
			}
		}

		// Reverse the letters slice in-place.
		for i := 0; i < length/2; i++ {
			j := length - 1 - i
			letters[i], letters[j] = letters[j], letters[i]
		}

		// Write the reversed letters back, re-applying the original casing
		// pattern by position.
		for i := 0; i < length; i++ {
			r := letters[i]
			switch cases[i] {
			case caseUpper:
				r = unicode.ToUpper(r)
			case caseLower:
				r = unicode.ToLower(r)
			case caseOther:
				// Leave r as-is (digits, symbols, etc.).
			}
			runes[start+i] = r
		}
	}

	// Scan the rune slice and reverse every contiguous run of "word" runes.
	wordStart := -1
	for i := 0; i <= n; i++ {
		if i < n && isWordRune(runes[i]) {
			// We are inside one word.
			if wordStart == -1 {
				wordStart = i
			}
		} else {
			// We just reached the end of a word (or are between words).
			if wordStart != -1 {
				reverseSegment(wordStart, i)
				wordStart = -1
			}
		}
	}

	return textual.UTF8String(runes)
}
