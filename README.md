# textual

![](assets/logo.png)

`textual` is a small Go toolkit for building **streaming** text‑processing pipelines.

It focuses on:

- **Streaming**: process text progressively as it arrives (readers, sockets, pipes, scanners…).
- **Composition**: chain stages, route items to multiple stages, merge results.
- **Encodings**: read/write many encodings while keeping an internal UTF‑8 representation.
- **Metadata‑friendly**: pipelines are generic and can carry either plain strings, richer objects, or structured payloads (JSON / CSV / XML).
- **Error propagation**: processors can attach non‑fatal, per‑item errors to the flowing values without breaking the stream.

The library is used by higher‑level projects like Tipa, but it can be used standalone in any Go program that needs robust incremental text processing.

---

## Installation

```bash
go get github.com/benoit-pereira-da-silva/textual
```

Import the core package (pipeline primitives + built‑in carriers live together in this package):

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

### Carrier

Pipelines don’t hard‑code a single “message” type. Instead, stages are generic over a carrier type `S` that implements `textual.Carrier[S]`:

```go
type Carrier[S any] interface {
    // UTF8String returns the current UTF‑8 representation of the value.
    UTF8String() UTF8String

    // FromUTF8String creates a new carrier from a UTF‑8 token.
    // The receiver is treated as a prototype and must not depend on receiver state.
    FromUTF8String(s UTF8String) S

    // WithIndex / GetIndex attach and retrieve an ordering hint.
    WithIndex(index int) S
    GetIndex() int

    // WithError / GetError attach and retrieve a non‑fatal processing error.
    // This lets processors report per‑item issues while keeping the stream alive.
    WithError(err error) S
    GetError() error
}
```

This interface lets `textual`:

- build a value from a scanned token (`FromUTF8String`),
- attach ordering metadata (`WithIndex` / `GetIndex`),
- render any value back to UTF‑8 (`UTF8String`),
- propagate recoverable errors inside the data stream (`WithError` / `GetError`).

**Important note about errors:** carrier errors are *data*, not control‑flow. Most of the `textual` stack does not stop when `GetError() != nil`. It is up to your processors and/or the final consumer to decide how to handle error‑carrying items (route them, log them, drop them, etc.). For fatal conditions, use context cancellation or stop producing outputs.

### AggregatableCarrier

Some utilities (like `SyncApply`) need to merge multiple outputs back into one. For that, carriers can also implement `textual.AggregatableCarrier[S]`:

```go
type AggregatableCarrier[S any] interface {
    Carrier[S]

    // Aggregate combines multiple carrier values into a single value.
    Aggregate(items []S) S
}
```

---

## Built‑in carriers

### `textual.StringCarrier` (minimal)

Use `textual.StringCarrier` when you just want a streaming string pipeline:

- `Value` carries the UTF‑8 text.
- `Index` is optional ordering metadata (for stable aggregation).
- `Error` carries optional per‑item errors.

It’s the simplest way to build processors that transform tokens and emit tokens.

### `textual.Parcel` (partial transformations + variants)

Use `textual.Parcel` when a processor might only transform *parts* of the input, or when it needs to expose multiple candidates.

At a glance:

- `Text` is the original UTF‑8 text for the current item.
- `Fragments` describes transformed spans in that text.
  - each fragment has a rune‑based `(Pos, Len)` range into `Text`.
  - `Transformed` holds the transformed string for that span.
  - `Variant` can be used to represent alternative candidates for the same span.
- `RawTexts()` computes the unprocessed segments (the complement of `Fragments`).
- `UTF8String()` reconstructs a final string by interleaving fragments and raw segments in positional order.
- `Error` carries optional per‑item errors (processor failures, fallbacks, warnings…).

### `textual.JsonCarrier` (raw JSON carrier)

Use `textual.JsonCarrier` when your pipeline should carry **raw JSON values** rather than plain text.

- `Value` is the raw JSON bytes for one top‑level value (`json.RawMessage`).
- `Index` is optional ordering metadata (for stable aggregation).
- `Error` carries optional per‑item errors.

Aggregation concatenates multiple JSON values into a single JSON array:

```json
[ <value0>, <value1>, ... ]
```

This is useful when you process a stream of JSON objects/arrays and need to “fan‑in” back into one JSON value.

#### Casting JSON into a concrete type

Use the helper `textual.CastJson[T]` to unmarshal a `JsonCarrier` into a Go value:

```go
obj, err := textual.CastJson[MyStruct](jsonCarrier)
```

### `textual.CsvCarrier` (raw CSV record carrier)

Use `textual.CsvCarrier` when your pipeline should carry **CSV records** (one record per item).

- `Value` is the raw UTF‑8 CSV record (typically *without* the trailing record separator).
- `Index` is optional ordering metadata.
- `Error` carries optional per‑item errors.

Aggregation concatenates multiple CSV records into a single CSV text by joining records with `\n`
(after stably sorting by `Index`).

#### Parsing a CSV record into fields

Use `textual.CastCsvRecord` to parse a `CsvCarrier` into a `[]string` using Go’s standard `encoding/csv` rules:

```go
fields, err := textual.CastCsvRecord(csvCarrier)
```

If you need custom delimiter/quoting rules, parse `csvCarrier.Value` yourself with an `encoding/csv.Reader`.

### `textual.XmlCarrier` (raw XML element carrier)

Use `textual.XmlCarrier` when your pipeline should carry **top‑level XML elements** (one complete element per item).

- `Value` is the raw UTF‑8 XML fragment (typically one element, without an XML declaration).
- `Index` is optional ordering metadata.
- `Error` carries optional per‑item errors.

Aggregation concatenates multiple elements into a single XML document by wrapping them into a container element:

```xml
<items> ... </items>
```

No extra whitespace is inserted between items.

#### Unmarshaling XML into a concrete type

Use the helper `textual.CastXml[T]` to unmarshal a `XmlCarrier` into a Go value:

```go
v, err := textual.CastXml[MyXMLStruct](xmlCarrier)
```

---

## Processing stages

### Processor and ProcessorFunc

A `Processor[S]` consumes a stream of `S` values and produces a stream of `S` values.

```go
type Processor[S textual.Carrier[S]] interface {
    Apply(ctx context.Context, in <-chan S) <-chan S
}
```

`ProcessorFunc` lets you write stages as plain functions:

```go
echo := textual.ProcessorFunc[textual.StringCarrier](
    func(ctx context.Context, in <-chan textual.StringCarrier) <-chan textual.StringCarrier {
        out := make(chan textual.StringCarrier)
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
                    out <- s // pass‑through
                }
            }
        }()
        return out
    },
)
```

#### Reporting a non‑fatal error from a processor

Instead of aborting the whole stream, a processor can attach an error to the item:

```go
validator := textual.ProcessorFunc[textual.StringCarrier](
    func(ctx context.Context, in <-chan textual.StringCarrier) <-chan textual.StringCarrier {
        out := make(chan textual.StringCarrier)
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
                    if len(s.Value) == 0 {
                        s = s.WithError(fmt.Errorf("empty token"))
                    }
                    out <- s
                }
            }
        }()
        return out
    },
)
```

Downstream stages can inspect `GetError()` (or route based on it).

### Transcoder and TranscoderFunc

A `Transcoder[S1,S2]` consumes a stream of `S1` and produces a stream of `S2`.

```go
type Transcoder[S1 textual.Carrier[S1], S2 textual.Carrier[S2]] interface {
    Apply(ctx context.Context, in <-chan S1) <-chan S2
}
```

`TranscoderFunc` is the functional adapter, just like `ProcessorFunc`.

---

## Streaming adapters

### IOReaderProcessor

`IOReaderProcessor` connects an `io.Reader` to a `Processor` using a `bufio.Scanner`.

- it scans tokens using a `bufio.SplitFunc` (default: `bufio.ScanLines`),
- it turns each token into an `S` using `FromUTF8String(token).WithIndex(i)`,
- it streams those values into the processor.

```go
reader := strings.NewReader("Hello, world!\n")

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

### IOReaderTranscoder

`IOReaderTranscoder` is the equivalent adapter for a `Transcoder[S1,S2]`.

---

## Composition

### Chain

`Chain` composes processors in sequence.

```go
chain := textual.NewChain(procA, procB, procC)
ioProc := textual.NewIOReaderProcessor(chain, reader)
```

The output of each stage is fed into the next stage.

### Glue

Sometimes you want to compose a `Transcoder` and a `Processor` into a single stage.

`Glue` is a small helper that builds a new `Transcoder`:

- `StickLeft`: `Transcoder[S1,S2]` then `Processor[S2]` ⇒ `Transcoder[S1,S2]`
- `StickRight`: `Processor[S1]` then `Transcoder[S1,S2]` ⇒ `Transcoder[S1,S2]`

This keeps the public API small while avoiding deeply nested `Apply(...)` calls.

### Router

`Router` distributes items to one or more processors based on predicates and a routing strategy:

- first match
- broadcast
- round‑robin
- random

Example with `textual.Parcel` (routing based on remaining raw text and per‑item errors):

```go
router := textual.NewRouter[textual.Parcel](textual.RoutingStrategyFirstMatch)

// 1) Errors first.
router.AddRoute(
    func(ctx context.Context, p textual.Parcel) bool {
        return p.GetError() != nil
    },
    errorHandlingProcessor,
)

// 2) Then items that still have unprocessed segments.
router.AddRoute(
    func(ctx context.Context, p textual.Parcel) bool {
        return len(p.RawTexts()) > 0
    },
    dictionaryProcessor,
)

// 3) Fallback route.
router.AddRoute(nil, loggingProcessor)
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

## Transformations

`Transformation` binds:

- a processor,
- an input “nature” (`Dialect` + `EncodingID`),
- an output “nature”.

`Process` handles decoding → processing → encoding:

```go
tr := textual.NewTransformation(
    "echo",
    echoProcessor,
    textual.Nature{Dialect: "plain", EncodingID: textual.UTF8},
    textual.Nature{Dialect: "plain", EncodingID: textual.UTF8},
)

if err := tr.Process(ctx, inputReadCloser, outputWriteCloser); err != nil {
    // handle error
}
```

`Process` encodes the **UTF‑8 rendering** of each output value (`res.UTF8String()`).
It does not interpret carrier errors: if you want to stop on per‑item errors,
your processor should do so explicitly (or the consumer should inspect `GetError()`).

---

## Tokenization helpers

### ScanExpression

In addition to standard `bufio` split functions, `textual` provides `ScanExpression`, which groups:

```text
[optional leading whitespace][non-whitespace run][optional trailing whitespace]
```

This is useful for “word‑by‑word” streaming while preserving punctuation and layout.

### ScanJSON

`ScanJSON` is a `bufio.SplitFunc` that frames a stream into **top‑level JSON values**:

- It ignores any leading bytes before the first `{` or `[` (spaces, newlines, commas, transport delimiters…).
- It tracks nesting and recognizes JSON strings (braces/brackets inside strings do not affect nesting).
- If EOF happens while a JSON value is still open, it returns `io.ErrUnexpectedEOF`.

### ScanCSV

`ScanCSV` is a `bufio.SplitFunc` that frames a stream into **CSV records**:

- It treats record separators (`\n`, `\r\n`, or `\r`) as record boundaries **only when outside quotes**.
- It recognizes CSV escaped quotes (`""`) inside quoted fields.
- If EOF happens while a quoted field is still open, it returns `io.ErrUnexpectedEOF`.

This split func does not validate the full CSV dialect (delimiter, comments, etc.); it provides robust framing so that each token is “one record”.

### ScanXML

`ScanXML` is a `bufio.SplitFunc` that frames a stream into **top‑level XML elements**:

- It ignores leading bytes before the first start element tag (`<name ...>`).
- It understands and skips XML comments (`<!-- ... -->`), CDATA (`<![CDATA[ ... ]]>`),
  processing instructions (`<? ... ?>`) and directives/doctype (`<! ... >`) while tracking nesting.
- It returns one complete element (start tag → matching end tag) as a token.
- If EOF happens while an element is still open, it returns `io.ErrUnexpectedEOF`.

This split func is a framing helper; it is not a full validating XML parser.

---

## Encoding helpers

The `encoding.go` module provides helpers to go from and to many encodings:

- `EncodingID` / `ParseEncoding`
- `NewUTF8Reader` (stream decode to UTF‑8)
- `ToUTF8` / `ReaderToUTF8`
- `FromUTF8` / `FromUTF8ToWriter`

Example:

```go
encoded := []byte{0x43, 0x61, 0x66, 0xE9} // "Café" in ISO-8859-1
s, _ := textual.ToUTF8(encoded, textual.ISO8859_1)
fmt.Println(s) // Café
```

## License

Apache 2.0. See `LICENSE`.
