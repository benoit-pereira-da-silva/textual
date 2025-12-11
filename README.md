# textual

`textual` is a small Go toolkit for building streaming text-processing
pipelines.

It focuses on:

- **Streaming** – process text progressively as it arrives.
- **Composition** – chain and route processing stages.
- **Encodings** – read and write many character encodings while keeping an
  internal UTF‑8 representation.
- **Transformations** – describe end‑to‑end text conversions with metadata.

The library is used by higher‑level projects but can be
used standalone in any Go program that needs robust, incremental text
processing.

---

## Installation

```shell
go get github.com/benoit-pereira-da-silva/textual
```

In code, import the core package:

```go
import textual "github.com/benoit-pereira-da-silva/textual/pkg/textual"
```

---

## Core concepts

### UTF8String and Result

All internal text is represented as UTF‑8 via the `UTF8String` type:

```go
type UTF8String string
```

A `Result` is the unit that flows through a pipeline:

```go
type Result struct {
    Index     int        // optional index in a stream
    Text      UTF8String // original or transformed text
    Fragments []Fragment // processed sub-parts (e.g. phonetic forms)
    Error     error      // optional error
}
```

`Result.Render()` recombines transformed fragments and untouched “raw” text
into a final string.

### Processor and ProcessorFunc

A `Processor` consumes a stream of `Result` values from an input channel and
produces processed `Result` values on an output channel:

```go
type Processor interface {
    Apply(ctx context.Context, in <-chan textual.Result) <-chan textual.Result
}
```

Most custom processing stages can be written as a simple function using
`ProcessorFunc`:

```go
echo := textual.ProcessorFunc(func(ctx context.Context, in <-chan textual.Result) <-chan textual.Result {
    out := make(chan textual.Result)
    go func() {
        defer close(out)
        for {
            select {
            case <-ctx.Done():
                return
            case res, ok := <-in:
                if !ok {
                    return
                }
                // process res as needed; here we just echo it
                out <- res
            }
        }
    }()
    return out
})
```

### IOReaderProcessor

`IOReaderProcessor` plugs an `io.Reader` into a `Processor` using a
`bufio.SplitFunc`:

```go
reader := strings.NewReader("Hello, world!\n")
proc   := echo                               // any textual.Processor
ioProc := textual.NewIOReaderProcessor(proc, reader)

// Optional: customise context and tokenization.
ioProc.SetContext(ctx)                // defaults to context.Background()
ioProc.SetSplitFunc(bufio.ScanLines)  // default; can be changed

out := ioProc.Start()
for res := range out {
    fmt.Println(res.Render())
}
```

This is especially convenient for log files, sockets, pipes, etc.

### Chain

`Chain` composes several processors in sequence:

```go
chain := textual.NewChain(procA, procB, procC)
ioProc := textual.NewIOReaderProcessor(chain, reader)
```

The output of each processor is fed to the next one.

### Router

`Router` distributes `Result` values to one or more processors based on
predicates and a routing strategy (first match, broadcast, round‑robin,
random):

```go
router := textual.NewRouter(textual.RoutingStrategyFirstMatch)

router.AddRoute(
    func(ctx context.Context, res textual.Result) bool {
        // route Results that still have raw text
        return len(res.RawTexts()) > 0
    },
    dictProcessor,
)

router.AddRoute(
    nil, // always eligible
    loggingProcessor,
)
```

### Encoding helpers

The `encoding.go` module provides helpers to go from and to many encodings:

- `EncodingID` – enum-like type for encodings (UTF‑8, UTF‑16, ISO‑8859‑*, …)
- `ParseEncoding` – parse a string (e.g. `"ISO-8859-1"`) into an `EncodingID`
- `NewUTF8Reader` – stream‑decode from a source encoding into UTF‑8
- `ToUTF8` / `ReaderToUTF8` – convert arbitrary encodings to UTF‑8
- `FromUTF8` / `FromUTF8ToWriter` – encode UTF‑8 into a target encoding

Example:

```go
package main

import (
    "bytes"
    "fmt"

    textual "github.com/benoit-pereira-da-silva/textual/pkg/textual"
)

func main() {
    // "Café" encoded as ISO-8859-1
    encoded := []byte{0x43, 0x61, 0x66, 0xE9}

    r := bytes.NewReader(encoded)
    s, err := textual.ReaderToUTF8(r, textual.ISO8859_1)
    if err != nil {
        panic(err)
    }
    fmt.Println(s) // Café
}
```

### Transformations

A `Transformation` binds a processor with information about the input and
output “nature” (dialect + encoding):

```go
tr := textual.NewTransformation[textual.Processor](
    "echo-utf8",
    echoProcessor,
    textual.Nature{Dialect: "plain", EncodingID: textual.UTF8},
    textual.Nature{Dialect: "plain", EncodingID: textual.UTF8},
)

if err := tr.Process(ctx, inputReadCloser, outputWriteCloser); err != nil {
    // handle error
}
```

`Process` takes care of:

- decoding input bytes to UTF‑8,
- running the processor,
- encoding the resulting text back to the requested encoding.

---

## Tokenization helpers: ScanExpression

In addition to the standard `bufio` split functions, `textual` provides
`ScanExpression`:

```go
// ScanExpression groups a word, the punctuation around it, and the
// spaces / line breaks that surround it into a single token.
func ScanExpression(data []byte, atEOF bool) (advance int, token []byte, err error)
```

Each token looks like:

```text
[optional leading whitespace][non-whitespace run][optional trailing whitespace]
```

This is particularly handy for “word‑by‑word” streaming where you still want
to preserve punctuation and layout:

```go
package main

import (
    "bufio"
    "fmt"
    "strings"

    textual "github.com/benoit-pereira-da-silva/textual/pkg/textual"
)

func main() {
    input := "Hello, world!\n"
    scanner := bufio.NewScanner(strings.NewReader(input))
    scanner.Split(textual.ScanExpression)

    for scanner.Scan() {
        fmt.Print(scanner.Text())
    }
    // Output: "Hello, world!\n"
}
```

---

## Examples

The repository contains examples under `examples/`:

- [`examples/reverse_words`](examples/reverse_words) – streams an excerpt from
  Baudelaire and reverses each word while preserving punctuation and layout.
  It includes a `--word-by-word` mode that uses `textual.ScanExpression`.

You can run the reverse words example with:

```shell
cd examples/reverse_words
go run main.go --word-by-word
```

---

## License

Licensed under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file
for details.
