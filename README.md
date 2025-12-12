# textual

`textual` is a small Go toolkit for building **streaming** text-processing pipelines.

It focuses on:

- **Streaming**: process text progressively as it arrives (readers, sockets, pipes, scanners…).
- **Composition**: chain stages, route items to multiple stages, merge results.
- **Encodings**: read/write many encodings while keeping an internal UTF‑8 representation.
- **Metadata-friendly**: pipelines are **generic** and can carry either plain strings or richer “result” objects.

The library is used by higher-level projects like Tipa, but it can be used standalone in any Go program that needs robust incremental text processing.

---

## Installation

```bash
go get github.com/benoit-pereira-da-silva/textual
```

Import the core package:

```go
import textual "github.com/benoit-pereira-da-silva/textual/pkg/textual"
```

---

## The generic model

### UTF8String

Internally, `textual` represents text as UTF‑8.

```go
type UTF8String = string
```

It’s an alias (not a distinct type) used for expressivity: you may decode from other encodings at the edges, but once inside the pipeline everything is treated as UTF‑8.

### UTF8Stringer

Pipelines don’t hard-code a single “message” type anymore. Instead, stages are generic over a carrier type `S` that implements `UTF8Stringer[S]`:

```go
type UTF8Stringer[S any] interface {
    UTF8String() UTF8String
    FromUTF8String(s UTF8String) S
    WithIndex(index int) S
    GetIndex() int
    Aggregate(items []S) S
}
```

This interface lets `textual`:

- build a value from a scanned token (`FromUTF8String`),
- attach ordering metadata (`WithIndex` / `GetIndex`),
- re-compose multiple outputs back into one (`Aggregate`),
- render any value back to UTF‑8 (`UTF8String`).

Built-in implementations:

- `textual.String`: minimal carrier (just a `string` + `Index`).
- `textual.Result`: richer carrier for partial processing (fragments, variants, confidence, raw segments…).

You can implement your own carrier type if you want to keep extra metadata flowing through the pipeline.

### Built-in carriers

#### `textual.String` (minimal)

Use `textual.String` when you just want a streaming string pipeline:

- `Value` carries the UTF‑8 text.
- `Index` is optional ordering metadata (for stable aggregation).

It’s the simplest way to build processors that transform tokens and emit tokens.

#### `textual.Result` (partial transformations + variants)

Use `textual.Result` when a processor might only transform *parts* of the input, or when it needs to expose multiple candidates.

At a glance:

- `Text` is the original UTF‑8 text for the current item.
- `Fragments` describes transformed spans in that text.
  - each fragment has a rune-based `(Pos, Len)` range into `Text`.
  - `Transformed` holds the transformed string for that span.
  - `Variant` can be used to represent alternative candidates for the same span.
- `RawTexts()` computes the unprocessed segments (the complement of `Fragments`).
- `UTF8String()` reconstructs a final string by interleaving fragments and raw segments in positional order.

`Result` also carries an optional `Error` field so processors can propagate failures without breaking the stream.

---

## Processing stages

### Processor and ProcessorFunc

A `Processor[S]` consumes a stream of `S` values and produces a stream of `S` values.

```go
type Processor[S textual.UTF8Stringer[S]] interface {
    Apply(ctx context.Context, in <-chan S) <-chan S
}
```

`ProcessorFunc` lets you write stages as plain functions:

```go
echo := textual.ProcessorFunc[textual.String](
    func(ctx context.Context, in <-chan textual.String) <-chan textual.String {
        out := make(chan textual.String)
        go func() {
            defer close(out)
            for {
                select {
                case <-ctx.Done():
                    return
                case s, ok := <-in:
                    if !ok {
                        return
                    }
                    out <- s // pass-through
                }
            }
        }()
        return out
    },
)
```

### IOReaderProcessor

`IOReaderProcessor` connects an `io.Reader` to a `Processor` using a `bufio.Scanner`.

- it scans tokens using a `bufio.SplitFunc` (default: `bufio.ScanLines`),
- it turns each token into an `S` using `FromUTF8String(token).WithIndex(i)`,
- it streams those values into the processor.

```go
reader := strings.NewReader("Hello, world!\n")

// Build a pipeline working on textual.String.
ioProc := textual.NewIOReaderProcessor(echo, reader)

// Optional.
ioProc.SetContext(ctx)
ioProc.SetSplitFunc(bufio.ScanLines)

out := ioProc.Start()
for s := range out {
    fmt.Print(s.UTF8String())
}
```

If your input is not UTF‑8, wrap the reader first:

```go
utf8Reader, _ := textual.NewUTF8Reader(rawReader, textual.ISO8859_1)
ioProc := textual.NewIOReaderProcessor(echo, utf8Reader)
```

### Chain

`Chain` composes processors in sequence.

```go
chain := textual.NewChain(procA, procB, procC)
ioProc := textual.NewIOReaderProcessor(chain, reader)
```

The output of each stage is fed into the next stage.

### Router

`Router` distributes items to one or more processors based on predicates and a routing strategy:

- first match
- broadcast
- round‑robin
- random

Example with `textual.Result` (routing based on remaining raw text):

```go
router := textual.NewRouter[textual.Result](textual.RoutingStrategyFirstMatch)

router.AddRoute(
    func(ctx context.Context, res textual.Result) bool {
        // Route results that still have unprocessed segments.
        return len(res.RawTexts()) > 0
    },
    dictionaryProcessor,
)

router.AddRoute(nil, loggingProcessor) // always eligible
```

### SyncApply

`SyncApply` applies a processor to a single input value and collects all outputs.

- If the processor produces **0** outputs, the input is returned (pass‑through).
- If it produces **1** output, it is returned.
- If it produces **N>1** outputs, they are aggregated using `S.Aggregate`.

```go
out := textual.SyncApply(ctx, proc, in)
```

---

## Transformations (dialect + encoding)

`Transformation` binds:

- a processor,
- an input “nature” (`Dialect` + `EncodingID`),
- an output “nature”.

`Process` handles decoding → processing → encoding:

```go
tr := textual.NewTransformation[textual.Result](
    "echo",
    echoProcessor,
    textual.Nature{Dialect: "plain", EncodingID: textual.UTF8},
    textual.Nature{Dialect: "plain", EncodingID: textual.UTF8},
)

if err := tr.Process(ctx, inputReadCloser, outputWriteCloser); err != nil {
    // handle error
}
```

---

## Encoding helpers

The `encoding.go` module provides helpers to go from and to many encodings:

- `EncodingID` / `ParseEncoding`
- `NewUTF8Reader` (stream decode to UTF‑8)
- `ToUTF8` / `ReaderToUTF8`
- `FromUTF8` / `FromUTF8ToWriter`

Example:

```go
// "Café" encoded as ISO‑8859‑1
encoded := []byte{0x43, 0x61, 0x66, 0xE9}

r := bytes.NewReader(encoded)
s, err := textual.ReaderToUTF8(r, textual.ISO8859_1)
if err != nil {
    panic(err)
}
fmt.Println(s) // Café
```

---

## Tokenization helper: ScanExpression

In addition to standard `bufio` split functions, `textual` provides `ScanExpression`, which groups:

```text
[optional leading whitespace][non-whitespace run][optional trailing whitespace]
```

This is useful for “word-by-word” streaming while preserving punctuation and layout.

---

## Examples

Examples live under `examples/`.

- `examples/reverse_words`: streams an excerpt from Baudelaire and reverses each word while preserving punctuation/layout. It includes a `--word-by-word` mode that uses `textual.ScanExpression`.

---

## License

Licensed under the Apache License, Version 2.0. See the `LICENSE` file for details.
