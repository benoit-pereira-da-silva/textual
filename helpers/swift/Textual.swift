//
// Textual.swift
//
// Lightweight Swift utilities to work with "textual" Parcel objects in
// client-side code (iOS, macOS, watchOS, tvOS).
//
// This file mirrors the behaviour of the Go "textual" package for:
//
//   - Parcel.rawTexts()
//   - Parcel.render() / Parcel.utf8String()
//
// and exposes the same EncodingID catalogue and name lookup helpers as
// encoding.go.
//
// It also provides a minimal UTF8String helper mirroring Go's textual.String,
// useful when you only need plain UTF‑8 text + index + error in client code.
//
// In addition, it provides a minimal JSON carrier mirroring Go's textual.JSON
// plus a scanJSON tokenizer helper to split a byte stream into JSON values.
//
// It is transport-agnostic: it assumes you already receive Parcel-like JSON
// objects (for example from URLSession, WebSocket, or other networking code)
// and helps you manipulate them in your Swift client.
//
// ---------------------------------------------------------------------------
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

import Foundation

/// UTF8Text is used for expressivity: all strings are assumed to be UTF-8.
///
/// The Go package uses `type UTF8String = string` as a readability alias.
/// On the Swift side we keep using `String` for storage and expose this alias
/// for clarity in APIs.
public typealias UTF8Text = String

// MARK: - Minimal carrier: UTF8String (mirrors Go textual.String)

/// UTF8String is the minimal "carrier" helper, mirroring Go's `textual.String`.
///
/// Use it when you only need:
///   - a UTF‑8 string (`value`)
///   - an optional ordering hint (`index`)
///   - an optional, portable error string (`error`)
///
/// This helper is intentionally small and does NOT implement Parcel-like
/// fragment logic.
public struct UTF8String: Codable, Equatable {
    /// The UTF‑8 text.
    public var value: UTF8Text

    /// Optional index in a stream.
    public var index: Int

    /// Optional error string (portable across JSON clients).
    public var error: String?

    public init(
        value: UTF8Text,
        index: Int = 0,
        error: String? = nil
    ) {
        self.value = value
        self.index = index
        self.error = error
    }

    /// Returns the UTF‑8 text (Go: UTF8String()).
    public func utf8String() -> UTF8Text {
        return value
    }

    /// Creates a new UTF8String from a UTF‑8 token (Go: FromUTF8String()).
    ///
    /// The Swift helper keeps this as a value-level constructor for convenience.
    public func fromUTF8String(_ text: UTF8Text) -> UTF8String {
        return UTF8String(value: text, index: 0, error: nil)
    }

    /// Returns a copy with its index set (Go: WithIndex()).
    public func withIndex(_ index: Int) -> UTF8String {
        var copy = self
        copy.index = index
        return copy
    }

    /// Returns the stored index (Go: GetIndex()).
    public func getIndex() -> Int {
        return index
    }

    /// Returns a copy with an error merged (Go: WithError()).
    ///
    /// Errors are stored as plain strings for portability. When multiple errors
    /// are attached, they are concatenated with `; `.
    public func withError(_ err: String?) -> UTF8String {
        guard let err = err?.trimmingCharacters(in: .whitespacesAndNewlines), !err.isEmpty else {
            return self
        }
        var copy = self
        if let existing = copy.error, !existing.isEmpty {
            if existing != err {
                copy.error = existing + "; " + err
            }
        } else {
            copy.error = err
        }
        return copy
    }

    /// Returns the stored error (Go: GetError()).
    public func getError() -> String? {
        return error
    }

    /// Aggregates multiple UTF8String values into one.
    ///
    /// Behaviour mirrors Go's `textual.String.Aggregate` intent:
    ///   - Items are stably sorted by index.
    ///   - When indices are equal, `value` is used as a tie-breaker.
    ///   - The output index is reset to 0.
    ///   - Errors are merged into a single portable string.
    public func aggregate(_ items: [UTF8String]) -> UTF8String {
        // Stable sort: keep original order as a final tie-breaker.
        let indexed = items.enumerated().map { (offset: $0.offset, item: $0.element) }
        let sorted = indexed.sorted { a, b in
            if a.item.index != b.item.index {
                return a.item.index < b.item.index
            }
            if a.item.value != b.item.value {
                return a.item.value < b.item.value
            }
            return a.offset < b.offset
        }.map { $0.item }

        var out = ""
        out.reserveCapacity(sorted.reduce(0) { $0 + $1.value.count })

        var mergedError: String? = nil
        for it in sorted {
            out.append(it.value)
            if let e = it.error, !e.isEmpty {
                mergedError = (mergedError == nil) ? e : (mergedError == e ? mergedError : (mergedError! + "; " + e))
            }
        }

        return UTF8String(value: out, index: 0, error: mergedError)
    }
}

/// Convenience factory for the minimal UTF8String carrier helper.
///
/// Creates a base UTF8String with:
///   - value = given argument
///   - index = 0
///   - error = nil
@discardableResult
public func utf8String(_ text: UTF8Text) -> UTF8String {
    return UTF8String(value: text, index: 0, error: nil)
}


// MARK: - Minimal carrier: JSON (mirrors Go textual.JSON)

/// JSON is a minimal "carrier" helper mirroring Go's `textual.JSON`.
///
/// Use it when your pipeline transports raw JSON values (objects or arrays)
/// instead of plain UTF‑8 text.
///
/// Typical streaming use-case:
///   - Tokenize a byte stream into JSON values (see `scanJSON` below).
///   - Attach an ordering index.
///   - Aggregate multiple JSON values into a single JSON array when needed.
///
/// The `value` property holds the raw JSON text (UTF‑8).
/// This helper does NOT parse or validate JSON; it only transports it.
public struct JSON: Codable, Equatable {
    /// The raw JSON value as UTF‑8 text (for example `{"a":1}` or `[1,2]`).
    public var value: UTF8Text

    /// Optional index in a stream.
    public var index: Int

    /// Optional error string (portable across JSON clients).
    public var error: String?

    public init(
        value: UTF8Text,
        index: Int = 0,
        error: String? = nil
    ) {
        self.value = value
        self.index = index
        self.error = error
    }

    /// Returns the raw JSON text (Go: UTF8String()).
    public func utf8String() -> UTF8Text {
        return value
    }

    /// Creates a new JSON carrier from a UTF‑8 token (Go: FromUTF8String()).
    public func fromUTF8String(_ text: UTF8Text) -> JSON {
        return JSON(value: text, index: 0, error: nil)
    }

    /// Returns a copy with its index set (Go: WithIndex()).
    public func withIndex(_ index: Int) -> JSON {
        var copy = self
        copy.index = index
        return copy
    }

    /// Returns the stored index (Go: GetIndex()).
    public func getIndex() -> Int {
        return index
    }

    /// Returns a copy with an error merged (Go: WithError()).
    ///
    /// Errors are stored as plain strings for portability. When multiple errors
    /// are attached, they are concatenated with `; `.
    public func withError(_ err: String?) -> JSON {
        guard let err = err?.trimmingCharacters(in: .whitespacesAndNewlines), !err.isEmpty else {
            return self
        }
        var copy = self
        if let existing = copy.error, !existing.isEmpty {
            if existing != err {
                copy.error = existing + "; " + err
            }
        } else {
            copy.error = err
        }
        return copy
    }

    /// Returns the stored error (Go: GetError()).
    public func getError() -> String? {
        return error
    }

    /// Aggregates multiple JSON values into a single JSON array.
    ///
    /// Behaviour mirrors Go's `textual.JSON.Aggregate` intent:
    ///   - Items are stably sorted by index.
    ///   - When indices are equal, `value` is used as a tie-breaker.
    ///   - The output index is reset to 0.
    ///   - Errors are merged into a single portable string.
    ///
    /// Important: no JSON validation is performed; `value` strings are inserted
    /// as-is into the output array.
    public func aggregate(_ items: [JSON]) -> JSON {
        // Stable sort: keep original order as a final tie-breaker.
        let indexed = items.enumerated().map { (offset: $0.offset, item: $0.element) }
        let sorted = indexed.sorted { a, b in
            if a.item.index != b.item.index {
                return a.item.index < b.item.index
            }
            if a.item.value != b.item.value {
                return a.item.value < b.item.value
            }
            return a.offset < b.offset
        }.map { $0.item }

        var out = "["
        // Rough capacity estimate.
        out.reserveCapacity(sorted.reduce(2) { $0 + $1.value.count + 1 })

        var mergedError: String? = nil
        for (i, it) in sorted.enumerated() {
            if i > 0 {
                out.append(",")
            }
            out.append(it.value)
            if let e = it.error, !e.isEmpty {
                mergedError = (mergedError == nil) ? e : (mergedError == e ? mergedError : (mergedError! + "; " + e))
            }
        }
        out.append("]")

        return JSON(value: out, index: 0, error: mergedError)
    }
}

/// Convenience factory for the minimal JSON carrier helper.
///
/// Creates a base JSON with:
///   - value = given argument
///   - index = 0
///   - error = nil
@discardableResult
public func rawJSON(_ text: UTF8Text) -> JSON {
    return JSON(value: text, index: 0, error: nil)
}

// MARK: - Core data types (rich carrier): Parcel / Fragment / RawText

/// Fragment represents a transformed portion of the original text.
///
/// Positions (`pos`) and lengths (`len`) are expressed in Unicode scalar
/// indices, not bytes. This matches the Go implementation which operates
/// in rune space.
///
/// Transformed text typically carries the processed representation, e.g.
/// IPA, SAMPA, pseudo-phonetics, etc.
public struct Fragment: Codable, Equatable {
    /// The transformed text (IPA, SAMPA, etc.).
    public var transformed: UTF8Text

    /// First scalar position in the original text.
    public var pos: Int

    /// Length in scalars in the original text.
    public var len: Int

    /// Optional confidence indicator for the transformation.
    public var confidence: Double

    /// Optional variant index when several candidates exist.
    public var variant: Int

    public init(
        transformed: UTF8Text,
        pos: Int,
        len: Int,
        confidence: Double = 0,
        variant: Int = 0
    ) {
        self.transformed = transformed
        self.pos = pos
        self.len = len
        self.confidence = confidence
        self.variant = variant
    }
}

/// RawText represents an untouched segment of the original text.
///
/// It is computed from a Parcel and covers every range that is not overlapped
/// by any Fragment (after merging overlapping fragments).
public struct RawText: Codable, Equatable {
    /// The raw text content.
    public var text: UTF8Text

    /// First scalar position in the original text.
    public var pos: Int

    /// Length in scalars.
    public var len: Int

    public init(text: UTF8Text, pos: Int, len: Int) {
        self.text = text
        self.pos = pos
        self.len = len
    }
}

/// Parcel is the rich value in the textual pipeline.
///
/// It mirrors the Go struct (textual.Parcel):
///
///   type Parcel struct {
///     Index     int
///     Text      UTF8String
///     Fragments []Fragment
///     Error     error
///   }
///
/// In Swift:
///   - `text` is a UTF-8 String.
///   - `fragments` is an array of Fragment values.
///   - `error` is an optional string; this helper does not interpret it.
public struct Parcel: Codable, Equatable {
    /// Optional index in a stream of parcels.
    public var index: Int

    /// Original UTF-8 text.
    public var text: UTF8Text

    /// Transformed fragments that reference ranges in `text`.
    public var fragments: [Fragment]

    /// Optional error. Kept as plain text for portability.
    public var error: String?

    public init(
        index: Int = -1,
        text: UTF8Text,
        fragments: [Fragment] = [],
        error: String? = nil
    ) {
        self.index = index
        self.text = text
        self.fragments = fragments
        self.error = error
    }

    // MARK: - Carrier-like convenience helpers

    /// Returns the UTF‑8 rendering (Go: UTF8String()).
    public func utf8String() -> UTF8Text {
        return render()
    }

    /// Creates a new Parcel from a UTF‑8 token (Go: FromUTF8String()).
    public func fromUTF8String(_ text: UTF8Text) -> Parcel {
        return Parcel(index: -1, text: text, fragments: [], error: nil)
    }

    /// Returns a copy with its index set (Go: WithIndex()).
    public func withIndex(_ index: Int) -> Parcel {
        var copy = self
        copy.index = index
        return copy
    }

    /// Returns the stored index (Go: GetIndex()).
    public func getIndex() -> Int {
        return index
    }

    /// Returns a copy with an error merged (Go: WithError()).
    ///
    /// Errors are stored as plain strings for portability. When multiple errors
    /// are attached, they are concatenated with `; `.
    public func withError(_ err: String?) -> Parcel {
        guard let err = err?.trimmingCharacters(in: .whitespacesAndNewlines), !err.isEmpty else {
            return self
        }
        var copy = self
        if let existing = copy.error, !existing.isEmpty {
            if existing != err {
                copy.error = existing + "; " + err
            }
        } else {
            copy.error = err
        }
        return copy
    }

    /// Returns the stored error (Go: GetError()).
    public func getError() -> String? {
        return error
    }

    /// Aggregates multiple parcels into a single parcel by concatenating their
    /// `text` fields and offsetting fragment positions.
    ///
    /// This is a convenience that mirrors the intent of Go Carrier.Aggregate for
    /// the Parcel shape:
    ///   - texts are concatenated in the provided order
    ///   - fragment positions are shifted by the Unicode scalar length of the
    ///     preceding texts
    ///   - errors are merged into a single portable string
    public func aggregate(_ parcels: [Parcel]) -> Parcel {
        var outText = ""
        outText.reserveCapacity(parcels.reduce(0) { $0 + $1.text.count })

        var outFragments: [Fragment] = []
        outFragments.reserveCapacity(parcels.reduce(0) { $0 + $1.fragments.count })

        var offset = 0
        var mergedError: String? = nil

        for p in parcels {
            // Merge errors.
            if let e = p.error, !e.isEmpty {
                mergedError = (mergedError == nil) ? e : (mergedError == e ? mergedError : (mergedError! + "; " + e))
            }

            // Shift fragments.
            for f in p.fragments {
                outFragments.append(
                    Fragment(
                        transformed: f.transformed,
                        pos: f.pos + offset,
                        len: f.len,
                        confidence: f.confidence,
                        variant: f.variant
                    )
                )
            }

            outText.append(p.text)
            offset += p.text.unicodeScalars.count
        }

        return Parcel(index: -1, text: outText, fragments: outFragments, error: mergedError)
    }
}

// MARK: - Convenience construction

/// Constructs a base Parcel to be used as a starting point, mirroring the
/// JS `input(text)` factory.
///
/// - Parameter text: Original UTF-8 text.
/// - Returns: A Parcel with index = -1, no fragments, and no error.
@discardableResult
public func input(_ text: UTF8Text) -> Parcel {
    return Parcel(index: -1, text: text, fragments: [], error: nil)
}


// MARK: - Streaming tokenization helper: scanJSON (mirrors Go ScanJSON)

/// Errors produced by `scanJSON`.
public enum JSONScanError: Error, LocalizedError, Equatable {
    /// The stream ended while a JSON value was still open.
    case unexpectedEOF

    /// A closing delimiter was found while no opening delimiter was pending.
    case unexpectedClosing(byte: UInt8, index: Int)

    /// A closing delimiter did not match the current opening delimiter.
    case mismatchedClosing(byte: UInt8, expectedOpen: UInt8, index: Int)

    public var errorDescription: String? {
        switch self {
        case .unexpectedEOF:
            return "Unexpected EOF while scanning JSON"
        case .unexpectedClosing(let byte, let index):
            return "Unexpected closing \(JSONScanError.describe(byte)) at byte \(index)"
        case .mismatchedClosing(let byte, let expectedOpen, let index):
            return "Mismatched closing \(JSONScanError.describe(byte)) for \(JSONScanError.describe(expectedOpen)) at byte \(index)"
        }
    }

    private static func describe(_ byte: UInt8) -> String {
        switch byte {
        case 0x7B: return "'{'"
        case 0x7D: return "'}'"
        case 0x5B: return "'['"
        case 0x5D: return "']'"
        case 0x22: return "'\"'"
        default:
            // Printable ASCII range.
            if byte >= 0x20 && byte <= 0x7E {
                return "'" + String(UnicodeScalar(byte)) + "'"
            }
            return "0x" + String(byte, radix: 16, uppercase: true)
        }
    }
}

/// scanJSON tokenizes a buffer into a single top-level JSON value (object or array).
///
/// This is a Swift analogue of Go's `bufio.SplitFunc` used by `ScanJSON`.
///
/// - Parameters:
///   - data: The current buffer (UTF‑8 bytes).
///   - atEOF: Whether the stream has ended.
/// - Returns: A tuple with:
///   - advance: number of bytes to consume from `data`
///   - token: a `Data` slice containing the JSON value (when complete)
///   - error: non-nil on framing errors (mismatched delimiters, unexpected EOF)
///
/// Behavior:
///
///   - Any leading bytes before the first `{` or `[` are ignored (consumed).
///   - Once a `{` or `[` is found, nesting is tracked until the matching closing
///     delimiter is found.
///   - JSON strings are recognized; braces/brackets inside strings do not affect
///     nesting. Basic escape handling is implemented so `\"` does not end a string.
///   - If `atEOF` is true and a JSON value is still open, `error` is set to
///     `JSONScanError.unexpectedEOF`.
public func scanJSON(_ data: Data, atEOF: Bool) -> (advance: Int, token: Data?, error: Error?) {
    // No data and nothing more to read.
    if atEOF && data.isEmpty {
        return (advance: 0, token: nil, error: nil)
    }

    // Find the first '{' or '['. Everything before it is ignored.
    var start: Int? = nil
    for (i, b) in data.enumerated() {
        if b == 0x7B /* '{' */ || b == 0x5B /* '[' */ {
            start = i
            break
        }
    }

    guard let startIndex = start else {
        // No opening delimiter in the current buffer.
        // Since we explicitly ignore leading noise, we can safely consume the
        // whole buffer (even when !atEOF) to avoid unbounded growth.
        return (advance: data.count, token: nil, error: nil)
    }

    // Consume ignored leading bytes first so the caller can retry from the
    // opening delimiter on the next iteration.
    if startIndex > 0 {
        return (advance: startIndex, token: nil, error: nil)
    }

    // data[0] is '{' or '['.
    var stack: [UInt8] = []
    stack.reserveCapacity(8)
    stack.append(data[0])

    var inString = false
    var escaped = false

    // Start scanning right after the opening delimiter.
    var i = 1
    while i < data.count {
        let b = data[i]

        if inString {
            if escaped {
                escaped = false
                i += 1
                continue
            }
            if b == 0x5C /* '\' */ {
                escaped = true
                i += 1
                continue
            }
            if b == 0x22 /* '"' */ {
                inString = false
            }
            i += 1
            continue
        }

        // Outside of strings.
        if b == 0x22 /* '"' */ {
            inString = true
            i += 1
            continue
        }

        if b == 0x7B /* '{' */ || b == 0x5B /* '[' */ {
            stack.append(b)
            i += 1
            continue
        }

        if b == 0x7D /* '}' */ || b == 0x5D /* ']' */ {
            guard let top = stack.last else {
                return (advance: 0, token: nil, error: JSONScanError.unexpectedClosing(byte: b, index: i))
            }

            let matches = (b == 0x7D && top == 0x7B) || (b == 0x5D && top == 0x5B)
            guard matches else {
                return (advance: 0, token: nil, error: JSONScanError.mismatchedClosing(byte: b, expectedOpen: top, index: i))
            }

            // Pop.
            stack.removeLast()

            if stack.isEmpty {
                let end = i + 1
                return (advance: end, token: data.subdata(in: 0..<end), error: nil)
            }

            i += 1
            continue
        }

        i += 1
    }

    // Buffer ended before we found the matching closing delimiter.
    if atEOF {
        return (advance: 0, token: nil, error: JSONScanError.unexpectedEOF)
    }
    return (advance: 0, token: nil, error: nil)
}

/// Convenience overload that scans a UTF‑8 string buffer.
///
/// - Note: `advance` is expressed in **UTF‑8 bytes**, not Swift `String.Index`.
public func scanJSON(_ text: String, atEOF: Bool) -> (advance: Int, token: String?, error: Error?) {
    let data = Data(text.utf8)
    let res = scanJSON(data, atEOF: atEOF)
    if let tokenData = res.token {
        let tokenText = String(data: tokenData, encoding: .utf8) ?? ""
        return (advance: res.advance, token: tokenText, error: res.error)
    }
    return (advance: res.advance, token: nil, error: res.error)
}

// MARK: - RawTexts / Render

public extension Parcel {
    /// Computes the non-transformed segments of the original `text`.
    ///
    /// Behaviour is intentionally aligned with the Go implementation:
    ///
    ///   - If there are no fragments, returns a single `RawText` covering the
    ///     whole text (in scalar units).
    ///   - Fragments are copied, sorted by `pos`, and treated as a union of
    ///     ranges. Overlapping fragments or multiple variants at the same `pos`
    ///     are merged via a moving cursor.
    ///   - Zero-length fragments and fragments fully outside the text are
    ///     ignored.
    ///   - Out-of-range fragment bounds are clamped to `[0, textLength]`.
    ///
    /// Positions and lengths are interpreted in terms of Unicode scalars,
    /// matching Go runes for UTF-8 strings.
    ///
    /// - Returns: Array of `RawText` segments.
    func rawTexts() -> [RawText] {
        var raw: [RawText] = []

        let scalars = Array(text.unicodeScalars)
        let textLen = scalars.count

        guard textLen > 0 else {
            return raw
        }

        guard !fragments.isEmpty else {
            // No fragments: the whole text is raw.
            raw.append(
                RawText(text: text, pos: 0, len: textLen)
            )
            return raw
        }

        // Copy and sort fragments by start position to compute the union of
        // their covered ranges in a single pass.
        var sortedFragments = fragments
        sortedFragments.sort { a, b in
            if a.pos == b.pos {
                return a.len < b.len
            }
            return a.pos < b.pos
        }

        // Cursor points to the first scalar index that has not yet been
        // classified as belonging to a fragment.
        var cursor = 0

        for fragment in sortedFragments {
            guard fragment.len > 0 else {
                // Ignore zero or negative length fragments.
                continue
            }

            var start = fragment.pos
            var end = fragment.pos + fragment.len

            // Clamp fragment bounds to the valid [0, textLen] interval.
            if start < 0 {
                start = 0
            }
            if start >= textLen {
                // Starts beyond the end of the text: nothing to do.
                continue
            }
            if end > textLen {
                end = textLen
            }

            // Any gap between cursor and the start of the fragment is raw text.
            if cursor < start {
                let slice = scalars[cursor..<start]
                let segmentText = String(String.UnicodeScalarView(slice))
                raw.append(
                    RawText(
                        text: segmentText,
                        pos: cursor,
                        len: start - cursor
                    )
                )
            }

            // Advance cursor to end of fragment, never backwards. This merges
            // overlapping fragments or multiple variants starting at the same
            // position.
            if cursor < end {
                cursor = end
            }
        }

        // Trailing text after the last fragment is also raw.
        if cursor < textLen {
            let slice = scalars[cursor..<textLen]
            let segmentText = String(String.UnicodeScalarView(slice))
            raw.append(
                RawText(
                    text: segmentText,
                    pos: cursor,
                    len: textLen - cursor
                )
            )
        }

        return raw
    }

    /// Reconstructs a single output string by merging transformed fragments and
    /// raw text segments according to their positions.
    ///
    /// Rules (matching the Go behaviour and the JS helper):
    ///
    ///   - Both fragments and raw texts reference absolute positions in the
    ///     original string.
    ///   - All segments (fragments + raw) are collected with their start `pos`.
    ///   - Segments are sorted by `pos` to restore the original sequence.
    ///   - Fragment output uses `Fragment.transformed`.
    ///   - RawText output uses `RawText.text`.
    ///   - No further transformation is applied to the text content.
    ///
    /// If multiple fragments share the same starting position, only the first
    /// one (in the original fragments array) is used, which is consistent with
    /// the Go implementation.
    ///
    /// - Returns: Reconstructed UTF-8 string.
    func render() -> UTF8Text {
        struct Segment {
            let pos: Int
            let text: UTF8Text
        }

        var segments: [Segment] = []

        let rawSegments = rawTexts()

        // Convert fragments into segments, only one per starting position.
        var lastFragPos: Int? = nil
        for fragment in fragments {
            if lastFragPos != fragment.pos {
                segments.append(
                    Segment(
                        pos: fragment.pos,
                        text: fragment.transformed
                    )
                )
                lastFragPos = fragment.pos
            }
        }

        // Convert raw text segments into segments as well.
        for raw in rawSegments {
            segments.append(
                Segment(
                    pos: raw.pos,
                    text: raw.text
                )
            )
        }

        // Sort by position to ensure the correct ordering.
        segments.sort { $0.pos < $1.pos }

        // Merge segments into the final output string.
        var builder = String()
        builder.reserveCapacity(text.count)

        for segment in segments {
            builder.append(segment.text)
        }

        return builder
    }
}

// MARK: - Encoding catalogue

/// EncodingID is an enum-like mapping of supported encodings to numeric IDs.
///
/// The numeric values follow the same ordering as the Go iota-based enum and
/// the JavaScript helper:
///
///   0  utf8
///   1  utf16le
///   2  utf16be
///   3  utf16leBom
///   4  utf16beBom
///   5  iso8859_1
///   ...
///   40 eucKr
///
public enum EncodingID: Int, Codable, CaseIterable {
    case utf8 = 0
    case utf16LE = 1
    case utf16BE = 2
    case utf16LEBOM = 3
    case utf16BEBOM = 4

    case iso8859_1 = 5
    case iso8859_2 = 6
    case iso8859_3 = 7
    case iso8859_4 = 8
    case iso8859_5 = 9
    case iso8859_6 = 10
    case iso8859_7 = 11
    case iso8859_8 = 12
    case iso8859_9 = 13
    case iso8859_10 = 14
    case iso8859_13 = 15
    case iso8859_14 = 16
    case iso8859_15 = 17
    case iso8859_16 = 18

    case koi8R = 19
    case koi8U = 20

    case windows874 = 21
    case windows1250 = 22
    case windows1251 = 23
    case windows1252 = 24
    case windows1253 = 25
    case windows1254 = 26
    case windows1255 = 27
    case windows1256 = 28
    case windows1257 = 29
    case windows1258 = 30

    case macRoman = 31
    case macCyrillic = 32

    case shiftJIS = 33
    case eucJP = 34
    case iso2022JP = 35

    case gbk = 36
    case hzgb2312 = 37
    case gb18030 = 38

    case big5 = 39

    case eucKR = 40

    /// Canonical encoding name, mirroring `EncodingName()` in Go.
    public var canonicalName: String {
        switch self {
        case .utf8: return "UTF-8"
        case .utf16LE: return "UTF-16LE"
        case .utf16BE: return "UTF-16BE"
        case .utf16LEBOM: return "UTF-16LE-BOM"
        case .utf16BEBOM: return "UTF-16BE-BOM"

        case .iso8859_1: return "ISO-8859-1"
        case .iso8859_2: return "ISO-8859-2"
        case .iso8859_3: return "ISO-8859-3"
        case .iso8859_4: return "ISO-8859-4"
        case .iso8859_5: return "ISO-8859-5"
        case .iso8859_6: return "ISO-8859-6"
        case .iso8859_7: return "ISO-8859-7"
        case .iso8859_8: return "ISO-8859-8"
        case .iso8859_9: return "ISO-8859-9"
        case .iso8859_10: return "ISO-8859-10"
        case .iso8859_13: return "ISO-8859-13"
        case .iso8859_14: return "ISO-8859-14"
        case .iso8859_15: return "ISO-8859-15"
        case .iso8859_16: return "ISO-8859-16"

        case .koi8R: return "KOI8-R"
        case .koi8U: return "KOI8-U"

        case .windows874: return "Windows-874"
        case .windows1250: return "Windows-1250"
        case .windows1251: return "Windows-1251"
        case .windows1252: return "Windows-1252"
        case .windows1253: return "Windows-1253"
        case .windows1254: return "Windows-1254"
        case .windows1255: return "Windows-1255"
        case .windows1256: return "Windows-1256"
        case .windows1257: return "Windows-1257"
        case .windows1258: return "Windows-1258"

        case .macRoman: return "MacRoman"
        case .macCyrillic: return "MacCyrillic"

        case .shiftJIS: return "ShiftJIS"
        case .eucJP: return "EUC-JP"
        case .iso2022JP: return "ISO-2022-JP"

        case .gbk: return "GBK"
        case .hzgb2312: return "HZ-GB2312"
        case .gb18030: return "GB18030"

        case .big5: return "Big5"

        case .eucKR: return "EUC-KR"
        }
    }

    /// Case-insensitive and trim-aware dictionary of encoding names to IDs,
    /// mirroring the Go `nameToEncoding` map and the JS `EncodingNameToId`.
    public static let nameToEncoding: [String: EncodingID] = {
        var map: [String: EncodingID] = [:]

        func add(_ name: String, _ id: EncodingID) {
            map[name.lowercased()] = id
        }

        add("utf-8", .utf8)
        add("utf8", .utf8)
        add("utf-16le", .utf16LE)
        add("utf-16be", .utf16BE)
        add("utf-16le-bom", .utf16LEBOM)
        add("utf-16be-bom", .utf16BEBOM)

        add("iso-8859-1", .iso8859_1)
        add("iso-8859-2", .iso8859_2)
        add("iso-8859-3", .iso8859_3)
        add("iso-8859-4", .iso8859_4)
        add("iso-8859-5", .iso8859_5)
        add("iso-8859-6", .iso8859_6)
        add("iso-8859-7", .iso8859_7)
        add("iso-8859-8", .iso8859_8)
        add("iso-8859-9", .iso8859_9)
        add("iso-8859-10", .iso8859_10)
        add("iso-8859-13", .iso8859_13)
        add("iso-8859-14", .iso8859_14)
        add("iso-8859-15", .iso8859_15)
        add("iso-8859-16", .iso8859_16)

        add("koi8-r", .koi8R)
        add("koi8-u", .koi8U)

        add("windows-874", .windows874)
        add("windows-1250", .windows1250)
        add("windows-1251", .windows1251)
        add("windows-1252", .windows1252)
        add("windows-1253", .windows1253)
        add("windows-1254", .windows1254)
        add("windows-1255", .windows1255)
        add("windows-1256", .windows1256)
        add("windows-1257", .windows1257)
        add("windows-1258", .windows1258)

        add("macroman", .macRoman)
        add("maccyrillic", .macCyrillic)

        add("shiftjis", .shiftJIS)
        add("shift-jis", .shiftJIS)
        add("euc-jp", .eucJP)
        add("iso-2022-jp", .iso2022JP)

        add("gbk", .gbk)
        add("hz-gb2312", .hzgb2312)
        add("gb18030", .gb18030)

        add("big5", .big5)

        add("euc-kr", .eucKR)

        return map
    }()

    /// Looks up an encoding ID from a human-readable name, mirroring
    /// `ParseEncoding(name string)` in Go and `parseEncoding(name)` in JS.
    ///
    /// The lookup is case-insensitive and ignores leading/trailing whitespace.
    ///
    /// - Parameter name: Name such as `"utf-8"` or `"Windows-1252"`.
    /// - Returns: Matching `EncodingID`.
    /// - Throws: `EncodingError.unknownEncoding` when the encoding is unknown.
    public static func parse(_ name: String) throws -> EncodingID {
        let trimmed = name.trimmingCharacters(in: .whitespacesAndNewlines)
        let key = trimmed.lowercased()
        if let id = nameToEncoding[key] {
            return id
        }
        throw EncodingError.unknownEncoding(trimmed)
    }
}

/// Errors thrown by encoding-related helpers.
public enum EncodingError: Error, LocalizedError {
    case unknownEncoding(String)

    public var errorDescription: String? {
        switch self {
        case .unknownEncoding(let value):
            return "Unknown encoding: \(value)"
        }
    }
}
