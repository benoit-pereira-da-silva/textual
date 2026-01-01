# textual-swift

Lightweight Swift utilities to work with values produced by the Go `textual` package.

This Swift helper intentionally mirrors the **current** Go implementation (no legacy/retro compatibility). It focuses on:

- Carrier data structures (`StringCarrier`, `JsonCarrier`, `JsonGenericCarrier<T>`, `CsvCarrier`, `XmlCarrier`, `Parcel`)
- `Parcel.rawTexts()` and `Parcel.render()` logic aligned with the Go `Parcel.RawTexts()` / `Parcel.UTF8String()`
- Streaming framing helpers equivalent to the Go `bufio.SplitFunc` helpers:
  `scanLines`, `scanExpression`, `scanJSON`, `scanCSV`, `scanXML`
- `EncodingID` catalogue + name lookup helpers aligned with Go `encoding.go`

No networking is implemented here. This module is transport-agnostic and only deals with in-memory values.

## Features

- ✅ Structs matching the Go `Parcel`, `Fragment`, and `RawText` layout.
- ✅ `rawTexts()` and `render()` logic aligned with the Go implementation.
- ✅ Carrier helpers matching the Go carriers:
  - `StringCarrier`
  - `JsonCarrier` (uses `RawJSON` to mirror Go `json.RawMessage` semantics)
  - `JsonGenericCarrier<T>`
  - `CsvCarrier`
  - `XmlCarrier`
- ✅ Framing helpers to split byte streams into tokens:
  - `scanLines` – keep trailing `\n` when present
  - `scanExpression` – whitespace + word core + trailing whitespace (word-centric tokens)
  - `scanJSON` – top-level JSON objects/arrays, with leading noise ignored
  - `scanCSV` – CSV records with correct handling of quoted fields
  - `scanXML` – top-level XML elements with robust framing
- ✅ `EncodingID` enum with the same numeric values as the Go version.
- ✅ `EncodingID.nameToEncoding` and `EncodingID.parse(_:)` helpers.
- ✅ `Codable` conformance for straightforward JSON decoding.

## Installation

Just drop `Textual.swift` into your project:

- Xcode: add the file to your app target.
- Swift Package: add it to one of your targets’ sources.

There are no external dependencies other than `Foundation`.

## Data model

```swift
public typealias UTF8String = String

public struct Fragment: Codable, Equatable {
    public var transformed: String
    public var pos: Int      // Unicode-scalar index in original text
    public var len: Int      // Unicode-scalar length
    public var confidence: Double
    public var variant: Int
}

public struct RawText: Codable, Equatable {
    public var text: String
    public var pos: Int
    public var len: Int
}

public struct Parcel: Codable, Equatable {
    public var index: Int          // -1 means unset (mirrors Go)
    public var text: String
    public var fragments: [Fragment]
    public var error: String?
}

public struct StringCarrier: Codable, Equatable {
    public var value: String
    public var index: Int
    public var error: String?
}

public struct RawJSON: Codable, Equatable {
    public var bytes: Data         // UTF-8 bytes for a JSON value
}

public struct JsonCarrier: Codable, Equatable {
    public var value: RawJSON      // mirrors Go `json.RawMessage`
    public var index: Int
    public var error: String?
}

public struct CsvCarrier: Codable, Equatable {
    public var value: String       // one CSV record (no trailing newline)
    public var index: Int
    public var error: String?
}

public struct XmlCarrier: Codable, Equatable {
    public var value: String       // one XML element fragment (UTF-8)
    public var index: Int
    public var error: String?
}
```

Positions and lengths in `Parcel` are expressed in **Unicode scalars**, which maps to Go’s `rune` indexing for UTF‑8 strings.

## Decoding from JSON

The struct layout is compatible with JSON produced by the Go backend.

```swift
import Foundation

let decoder = JSONDecoder()
let parcel = try decoder.decode(Parcel.self, from: data)
```

Example JSON:

```json
{
  "index": 0,
  "text": "Hello, café",
  "fragments": [
    { "transformed": "həˈloʊ", "pos": 0, "len": 5, "confidence": 0.9, "variant": 0 }
  ]
}
```

## RawTexts / Render

```swift
let p: Parcel = ...

let rawParts: [RawText] = p.rawTexts()
let rendered: String = p.render()
```

You can also create Parcels directly in Swift:

```swift
let p = input("Hello, café")     // index = -1, no fragments
let rendered = p.render()        // -> "Hello, café"
```

## Minimal StringCarrier

Use `StringCarrier` when you only need to carry plain UTF‑8 text with an index and an optional error string:

```swift
let s = StringCarrier(value: "plain token").withIndex(42)
print(s.utf8String()) // "plain token"
```

This mirrors the role of Go’s `textual.StringCarrier`.

## JSON carrier: JsonCarrier

`JsonCarrier` mirrors Go’s `textual.JsonCarrier`. Its `value` encodes/decodes as a **nested JSON value** (not a quoted string), matching Go `json.RawMessage`.

```swift
let j = JsonCarrier(value: RawJSON(utf8String: #"{"a":[1,2,3]}"#))
print(j.utf8String()) // {"a":[1,2,3]}
```

To decode a JSON value into a concrete type:

```swift
struct Payload: Decodable { let a: [Int] }

let payload: Payload = try CastJson(j)
```

## Framing helpers

All framing helpers operate on `Data` buffers and return:

- `advance`: how many **bytes** to remove from the front of your buffer
- `token`: the framed token, if available
- `error`: an error if framing failed

### scanJSON

Rules (mirrors Go `ScanJSON`):

- Everything before the first `{` or `[` is ignored (consumed).
- Nesting is tracked until the matching `}` or `]`.
- Quotes and escapes are handled so braces inside JSON strings don’t affect nesting.

Example:

```swift
var buffer = Data(" \n{\"a\":[1,2,{\"b\":\"x\"}]}{\"c\":3}".utf8)

while true {
    let res = scanJSON(buffer, atEOF: true)
    if let err = res.error { throw err }
    guard let token = res.token else { break }

    let jsonText = String(decoding: token, as: UTF8.self)
    print("json token:", jsonText)

    buffer.removeFirst(res.advance)
}
```

### scanCSV

`scanCSV` frames CSV records and correctly ignores newlines inside quoted fields.

```swift
var buffer = Data("a,b\r\n\"x\ny\",z\n".utf8)

while true {
    let res = scanCSV(buffer, atEOF: true)
    if let err = res.error { throw err }
    guard let token = res.token else { break }

    let record = String(decoding: token, as: UTF8.self)
    print("csv record:", record)

    buffer.removeFirst(res.advance)
}
```

### scanXML

`scanXML` frames one complete top-level XML element, ignoring leading prolog/PI/comments/doctypes.

```swift
var buffer = Data("<?xml version=\"1.0\"?><a><b/></a><c/>".utf8)

while true {
    let res = scanXML(buffer, atEOF: true)
    if let err = res.error { throw err }
    guard let token = res.token else { break }

    let xml = String(decoding: token, as: UTF8.self)
    print("xml element:", xml)

    buffer.removeFirst(res.advance)
}
```

## Encoding dictionary

`EncodingID` mirrors the Go `EncodingID` enum and uses the same numeric values:

```swift
let id = EncodingID.utf8
print(id.rawValue)      // 0
print(id.encodingName)  // "UTF-8"
```

Lookup by name:

```swift
let windows1252 = try EncodingID.parse("Windows-1252")
print(windows1252.rawValue) // 24
```

## Notes on indexing

- `Parcel.rawTexts()` and `Parcel.render()` assume all positions and lengths (`pos`, `len`) are expressed in units of **Unicode scalars**.
- If you compute fragment positions in Swift, prefer iterating over `text.unicodeScalars` to stay consistent with the Go side.

## License

Same license as the rest of the project (Apache 2.0 in the original Go code).
