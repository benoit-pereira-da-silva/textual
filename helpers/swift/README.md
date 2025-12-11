# textual-swift

Lightweight Swift utilities to work with `textual` `Result` objects in iOS / macOS clients.

This module is the client-side Swift counterpart of the Go `textual`:

- `Result.rawTexts()` – compute the raw segments of a text that are **not** covered by any fragment.
- `Result.render()` – merge transformed fragments and raw text back into a single output string.
- `EncodingID` catalogue – mirror of the Go `EncodingID` enum and `nameToEncoding` dictionary.

No networking or transcoding is implemented here on purpose: this file is
transport-agnostic and only deals with in-memory `Result` values.

## Features

- ✅ Structs matching the Go `Result`, `Fragment`, and `RawText` layout.
- ✅ `rawTexts()` and `render()` logic aligned with the Go implementation.
- ✅ `EncodingID` enum with the same numeric values as the Go / JS versions.
- ✅ `EncodingID.nameToEncoding` and `EncodingID.parse(_:)` helpers.
- ✅ `Codable` conformance for straightforward JSON decoding.

## Installation

Just drop `TextualStreamUtils.swift` into your project:

- Xcode: add the file to your app target.
- Swift Package: add it to one of your targets’ sources.

There are no external dependencies other than `Foundation`.

## Data model

```swift
public struct Fragment: Codable, Equatable {
    public var transformed: String
    public var pos: Int      // scalar index in original text
    public var len: Int      // scalar length
    public var confidence: Double
    public var variant: Int
}

public struct RawText: Codable, Equatable {
    public var text: String
    public var pos: Int
    public var len: Int
}

public struct Result: Codable, Equatable {
    public var index: Int
    public var text: String
    public var fragments: [Fragment]
    public var error: String?
}
```

Positions and lengths are expressed in **Unicode scalars**, which maps to Go’s
`rune` indexing for UTF‑8 strings.

## Decoding from JSON

The struct layout is compatible with the JSON produced by the Go backend.

```swift
import Foundation

// Assuming `data` is a Data value from URLSession or WebSocket.
let decoder = JSONDecoder()
let result = try decoder.decode(Result.self, from: data)
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
// Start from a decoded Result
let res: Result = ...

// Get the segments of the original text that have NOT been transformed.
let rawParts: [RawText] = res.rawTexts()

// Reconstruct the display string mixing transformed and raw segments.
let rendered: String = res.render()
```

You can also create Results directly in Swift, mirroring the Go `Input` helper:

```swift
let res = input("Hello, café")          // index = -1, no fragments
let rendered = res.render()             // -> "Hello, café"
```

## Encoding dictionary

`EncodingID` mirrors the Go `EncodingID` enum and the JS `EncodingID` object.
Each case has the same numeric value:

```swift
let id = EncodingID.utf8
print(id.rawValue)            // 0
print(id.canonicalName)       // "UTF-8"
```

You can also look up an encoding by name:

```swift
do {
    let utf8 = try EncodingID.parse("utf-8")
    let windows1252 = try EncodingID.parse("Windows-1252")

    print(utf8.canonicalName)       // "UTF-8"
    print(windows1252.rawValue)     // 24
} catch {
    print(error)    // Unknown encoding: ...
}
```

`EncodingID.parse(_:)` is a thin Swift equivalent of Go’s `ParseEncoding` and
the JS `parseEncoding(name)` helper.

## Notes on indexing

- `Result.rawTexts()` and `Result.render()` assume all positions and lengths
  (`pos`, `len`) are expressed in units of **Unicode scalars**.
- If you compute fragment positions in Swift, prefer iterating over
  `text.unicodeScalars` rather than `text.utf16` or `text.indices` to stay
  consistent with the Go side.

## License

Same license as the rest of the project (Apache 2.0 in the original Go code).
