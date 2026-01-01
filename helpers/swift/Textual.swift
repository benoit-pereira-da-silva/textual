//
// Textual.swift
//
// Lightweight Swift utilities to work with the Go "textual" package data model
// in client-side code (iOS, macOS, watchOS, tvOS).
//
// This file intentionally mirrors the behaviour of the Go "textual" package for:
//
//   - Carrier contracts (StringCarrier, JsonCarrier, JsonGenericCarrier<T>, CsvCarrier, XmlCarrier, Parcel)
//   - Parcel.rawTexts() and Parcel.utf8String() reconstruction
//   - Streaming framing helpers equivalent to Go bufio.SplitFunc implementations:
//       scanLines, scanExpression, scanJSON, scanCSV, scanXML
//   - EncodingID catalogue + name lookup helpers (mirrors encoding.go)
//
// It is transport-agnostic: it only deals with in-memory values.
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

// MARK: - UTF-8 alias (mirrors Go `type UTF8String = string`)

/// UTF8String is a symbolic alias used throughout the Go package.
///
/// In Go, `type UTF8String = string` exists mostly for readability.
/// In Swift, we keep `String` as the storage type and expose the same alias
/// for API clarity.
public typealias UTF8String = String

// MARK: - Carrier contract (Swift equivalent of Go `Carrier[S]`)

/// Carrier is the contract used by the generic Go pipeline.
///
/// Swift does not implement the Go pipeline (channels + context), but keeping a
/// matching carrier API makes it straightforward to manipulate values coming
/// from (or going to) Go services.
public protocol Carrier {
    /// Returns the current UTF-8 representation of this carrier.
    func utf8String() -> UTF8String

    /// Creates a new carrier from a UTF-8 token.
    ///
    /// The receiver is treated as a prototype, mirroring Go where `FromUTF8String`
    /// is often called on a zero value.
    func fromUTF8String(_ s: UTF8String) -> Self

    /// Returns a copy with its index set.
    func withIndex(_ index: Int) -> Self

    /// Returns the stored index.
    func getIndex() -> Int

    /// Returns a copy with an error merged.
    func withError(_ error: String?) -> Self

    /// Returns the stored error.
    func getError() -> String?
}

// MARK: - Zero initializable (used to mirror Go's "zero value" prototypes)

/// ZeroInitializable is a small helper protocol used to mirror Go’s “zero value”
/// behaviour for generic carriers.
///
/// In Go, every type has a zero value. In Swift, we model this as a requirement
/// for a public parameterless initializer.
///
/// Many standard library types already have `init()`. This file declares
/// conformances for common types so `JsonGenericCarrier<T>` can be used with
/// built-in payloads (String, Int, arrays, dictionaries…).
public protocol ZeroInitializable {
    init()
}

extension String: ZeroInitializable {}
extension Int: ZeroInitializable {}
extension Int64: ZeroInitializable {}
extension Double: ZeroInitializable {}
extension Bool: ZeroInitializable {}
extension Data: ZeroInitializable {}
extension Array: ZeroInitializable {}
extension Dictionary: ZeroInitializable {}

// MARK: - Shared helpers

private func normalizeError(_ err: String?) -> String? {
    guard let err = err?.trimmingCharacters(in: .whitespacesAndNewlines), !err.isEmpty else {
        return nil
    }
    return err
}

/// joinErrors mirrors the intent of Go's `errors.Join` for this Swift helper:
/// - nils are ignored
/// - duplicates are avoided
/// - errors are concatenated with a newline separator
private func joinErrors(_ a: String?, _ b: String?) -> String? {
    let aa = normalizeError(a)
    let bb = normalizeError(b)

    if aa == nil { return bb }
    if bb == nil { return aa }
    if aa == bb { return aa }

    return (aa ?? "") + "\n" + (bb ?? "")
}

private func stableSorted<T>(_ items: [T], by key: (T) -> Int) -> [T] {
    return items.enumerated().sorted { a, b in
        let ka = key(a.element)
        let kb = key(b.element)
        if ka != kb { return ka < kb }
        return a.offset < b.offset
    }.map { $0.element }
}

/// In the Go package, `Parcel.Index == -1` means "unset".
/// When sorting, we treat negative indices as "last" (Int.max) to keep unset
/// values in their original order after all indexed items.
private func orderingKey(_ index: Int) -> Int {
    return index < 0 ? Int.max : index
}

private func stringFromScalars(_ scalars: ArraySlice<UnicodeScalar>) -> UTF8String {
    var view = String.UnicodeScalarView()
    view.append(contentsOf: scalars)
    return String(view)
}

// MARK: - Carrier facilities (mirrors `carrier_facilities.go`)

public func StringFrom(_ s: UTF8String) -> StringCarrier { StringCarrier(value: s) }

public func JSONFrom(_ s: UTF8String) -> JsonCarrier { JsonCarrier(value: RawJSON(utf8String: s)) }

public func JSONCarrierFrom<T: Codable & ZeroInitializable>(_ s: UTF8String, _: T.Type = T.self) -> JsonGenericCarrier<T> {
    return JsonGenericCarrier<T>(value: T()).fromUTF8String(s)
}

public func CSVFrom(_ s: UTF8String) -> CsvCarrier { CsvCarrier(value: s) }

public func XMLFrom(_ s: UTF8String) -> XmlCarrier { XmlCarrier(value: s) }

public func ParcelFrom(_ s: UTF8String) -> Parcel { Parcel(index: -1, text: s, fragments: [], error: nil) }

// MARK: - StringCarrier (mirrors Go `textual.StringCarrier`)

/// StringCarrier is a minimal carrier transporting plain UTF-8 text.
///
/// Use it when you only need:
///   - a UTF-8 string (`value`)
///   - an optional ordering hint (`index`)
///   - an optional, per-item error (`error`)
public struct StringCarrier: Codable, Equatable, Carrier {
    public var value: UTF8String
    public var index: Int
    public var error: String?

    public init(value: UTF8String, index: Int = 0, error: String? = nil) {
        self.value = value
        self.index = index
        self.error = normalizeError(error)
    }

    public func utf8String() -> UTF8String { value }

    public func fromUTF8String(_ s: UTF8String) -> StringCarrier {
        return StringCarrier(value: s, index: 0, error: nil)
    }

    public func withIndex(_ index: Int) -> StringCarrier {
        var copy = self
        copy.index = index
        return copy
    }

    public func getIndex() -> Int { index }

    public func withError(_ error: String?) -> StringCarrier {
        var copy = self
        copy.error = joinErrors(copy.error, error)
        return copy
    }

    public func getError() -> String? { error }

    /// Aggregates multiple StringCarrier values into one by concatenating their values.
    ///
    /// Behaviour mirrors the Go `Aggregate` intent:
    ///   - Items are stably sorted by index.
    ///   - Output index is reset to 0.
    ///   - Errors are merged into a single portable string.
    public func aggregate(_ items: [StringCarrier]) -> StringCarrier {
        return StringCarrier.aggregate(items)
    }

    public static func aggregate(_ items: [StringCarrier]) -> StringCarrier {
        let sorted = stableSorted(items) { orderingKey($0.index) }

        var out = ""
        out.reserveCapacity(sorted.reduce(0) { $0 + $1.value.count })

        var mergedError: String? = nil
        for it in sorted {
            out.append(it.value)
            mergedError = joinErrors(mergedError, it.error)
        }

        return StringCarrier(value: out, index: 0, error: mergedError)
    }
}

// MARK: - CsvCarrier (mirrors Go `textual.CsvCarrier`)

/// CsvCarrier is a minimal carrier that transports an opaque CSV record value.
///
/// By convention, `value` should NOT include the trailing record separator (newline).
/// This matches the Go `ScanCSV` framing helper.
public struct CsvCarrier: Codable, Equatable, Carrier {
    public var value: UTF8String
    public var index: Int
    public var error: String?

    public init(value: UTF8String, index: Int = 0, error: String? = nil) {
        self.value = value
        self.index = index
        self.error = normalizeError(error)
    }

    public func utf8String() -> UTF8String { value }

    public func fromUTF8String(_ s: UTF8String) -> CsvCarrier {
        return CsvCarrier(value: s, index: 0, error: nil)
    }

    public func withIndex(_ index: Int) -> CsvCarrier {
        var copy = self
        copy.index = index
        return copy
    }

    public func getIndex() -> Int { index }

    public func withError(_ error: String?) -> CsvCarrier {
        var copy = self
        copy.error = joinErrors(copy.error, error)
        return copy
    }

    public func getError() -> String? { error }

    /// Aggregates multiple CsvCarrier values into a multi-record CSV text.
    ///
    /// Behaviour mirrors the Go documentation:
    ///   - Items are stably sorted by index.
    ///   - Records are joined with "\n".
    ///   - Output index is reset to 0.
    public func aggregate(_ items: [CsvCarrier]) -> CsvCarrier {
        return CsvCarrier.aggregate(items)
    }

    public static func aggregate(_ items: [CsvCarrier]) -> CsvCarrier {
        let sorted = stableSorted(items) { orderingKey($0.index) }

        var out = ""
        out.reserveCapacity(sorted.reduce(0) { $0 + $1.value.count + 1 })

        var mergedError: String? = nil
        for (i, it) in sorted.enumerated() {
            if i > 0 { out.append("\n") }
            out.append(it.value)
            mergedError = joinErrors(mergedError, it.error)
        }

        return CsvCarrier(value: out, index: 0, error: mergedError)
    }
}

// MARK: - XmlCarrier (mirrors Go `textual.XmlCarrier`)

/// XmlCarrier is a minimal carrier that transports an opaque XML fragment.
///
/// Intended usage:
///   - Each `value` contains one complete top-level XML element encoded as UTF-8.
///   - `scanXML` produces tokens with exactly that shape.
///
/// Aggregation (mirrors Go docs):
///   - values are stably sorted by index
///   - concatenated inside a container: "<items>...</items>"
public struct XmlCarrier: Codable, Equatable, Carrier {
    public var value: UTF8String
    public var index: Int
    public var error: String?

    public init(value: UTF8String, index: Int = 0, error: String? = nil) {
        self.value = value
        self.index = index
        self.error = normalizeError(error)
    }

    public func utf8String() -> UTF8String { value }

    public func fromUTF8String(_ s: UTF8String) -> XmlCarrier {
        return XmlCarrier(value: s, index: 0, error: nil)
    }

    public func withIndex(_ index: Int) -> XmlCarrier {
        var copy = self
        copy.index = index
        return copy
    }

    public func getIndex() -> Int { index }

    public func withError(_ error: String?) -> XmlCarrier {
        var copy = self
        copy.error = joinErrors(copy.error, error)
        return copy
    }

    public func getError() -> String? { error }

    public func aggregate(_ items: [XmlCarrier]) -> XmlCarrier {
        return XmlCarrier.aggregate(items)
    }

    public static func aggregate(_ items: [XmlCarrier]) -> XmlCarrier {
        let sorted = stableSorted(items) { orderingKey($0.index) }

        var out = "<items>"
        out.reserveCapacity(sorted.reduce(out.count + "</items>".count) { $0 + $1.value.count })

        var mergedError: String? = nil
        for it in sorted {
            out.append(it.value)
            mergedError = joinErrors(mergedError, it.error)
        }
        out.append("</items>")

        return XmlCarrier(value: out, index: 0, error: mergedError)
    }
}

// MARK: - RawJSON (supports Go `json.RawMessage` semantics in Swift)

/// JSONValue is a small Codable representation of any JSON value.
///
/// It is used internally by RawJSON to decode/encode raw JSON values as JSON,
/// rather than as base64-encoded Data.
public enum JSONValue: Codable, Equatable {
    case object([String: JSONValue])
    case array([JSONValue])
    case string(String)
    case int(Int64)
    case double(Double)
    case bool(Bool)
    case null

    public init(from decoder: Decoder) throws {
        let c = try decoder.singleValueContainer()

        if c.decodeNil() {
            self = .null
            return
        }

        if let b = try? c.decode(Bool.self) {
            self = .bool(b)
            return
        }

        if let i = try? c.decode(Int64.self) {
            self = .int(i)
            return
        }

        if let d = try? c.decode(Double.self) {
            self = .double(d)
            return
        }

        if let s = try? c.decode(String.self) {
            self = .string(s)
            return
        }

        if let arr = try? c.decode([JSONValue].self) {
            self = .array(arr)
            return
        }

        if let obj = try? c.decode([String: JSONValue].self) {
            self = .object(obj)
            return
        }

        throw DecodingError.dataCorruptedError(in: c, debugDescription: "Unsupported JSON value")
    }

    public func encode(to encoder: Encoder) throws {
        var c = encoder.singleValueContainer()
        switch self {
        case .null:
            try c.encodeNil()
        case .bool(let b):
            try c.encode(b)
        case .int(let i):
            try c.encode(i)
        case .double(let d):
            try c.encode(d)
        case .string(let s):
            try c.encode(s)
        case .array(let a):
            try c.encode(a)
        case .object(let o):
            try c.encode(o)
        }
    }

    /// Best-effort compact JSON string representation.
    public func compactString() -> UTF8String {
        do {
            let data = try JSONEncoder().encode(self)
            return String(decoding: data, as: UTF8.self)
        } catch {
            return error.localizedDescription
        }
    }
}

public enum RawJSONError: Error, LocalizedError, Equatable {
    case invalidRawJSON(String)

    public var errorDescription: String? {
        switch self {
        case .invalidRawJSON(let details):
            return "Invalid raw JSON: \(details)"
        }
    }
}

/// RawJSON stores UTF-8 JSON bytes and encodes/decodes as a raw JSON value.
///
/// This is the closest Swift analogue to Go's `json.RawMessage` in this project:
/// when embedded in another Codable container, RawJSON is encoded as JSON, not as base64.
public struct RawJSON: Codable, Equatable {
    public var bytes: Data

    public init(_ bytes: Data) {
        self.bytes = bytes
    }

    public init(utf8String: UTF8String) {
        self.bytes = Data(utf8String.utf8)
    }

    public func utf8String() -> UTF8String {
        return String(decoding: bytes, as: UTF8.self)
    }

    public func fromUTF8String(_ s: UTF8String) -> RawJSON {
        return RawJSON(utf8String: s)
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.singleValueContainer()
        let value = try c.decode(JSONValue.self)
        self.bytes = try JSONEncoder().encode(value)
    }

    public func encode(to encoder: Encoder) throws {
        do {
            let value = try JSONDecoder().decode(JSONValue.self, from: bytes)
            var c = encoder.singleValueContainer()
            try c.encode(value)
        } catch {
            throw RawJSONError.invalidRawJSON(error.localizedDescription)
        }
    }
}

// MARK: - JsonCarrier (mirrors Go `textual.JsonCarrier`)

/// JsonCarrier is a minimal carrier that transports an opaque JSON value.
///
/// In Go, `JsonCarrier.Value` is a `json.RawMessage` and encodes in JSON as a nested JSON value.
/// In Swift, we model that with `RawJSON`.
public struct JsonCarrier: Codable, Equatable, Carrier {
    public var value: RawJSON
    public var index: Int
    public var error: String?

    public init(value: RawJSON, index: Int = 0, error: String? = nil) {
        self.value = value
        self.index = index
        self.error = normalizeError(error)
    }

    /// Returns the raw JSON value as UTF-8 text (mirrors Go `UTF8String()`).
    public func utf8String() -> UTF8String { value.utf8String() }

    public func fromUTF8String(_ s: UTF8String) -> JsonCarrier {
        return JsonCarrier(value: RawJSON(utf8String: s), index: 0, error: nil)
    }

    public func withIndex(_ index: Int) -> JsonCarrier {
        var copy = self
        copy.index = index
        return copy
    }

    public func getIndex() -> Int { index }

    public func withError(_ error: String?) -> JsonCarrier {
        var copy = self
        copy.error = joinErrors(copy.error, error)
        return copy
    }

    public func getError() -> String? { error }

    /// Aggregates multiple JsonCarrier values into a single JSON array value.
    ///
    /// Behaviour mirrors the Go documentation intent:
    ///   - Items are stably sorted by index.
    ///   - The output `value` is a JSON array built by concatenating raw values.
    ///   - Output index is reset to 0.
    ///   - No JSON validation is performed when aggregating (values are inserted as-is).
    public func aggregate(_ items: [JsonCarrier]) -> JsonCarrier {
        return JsonCarrier.aggregate(items)
    }

    public static func aggregate(_ items: [JsonCarrier]) -> JsonCarrier {
        let sorted = stableSorted(items) { orderingKey($0.index) }

        var out = "["
        out.reserveCapacity(sorted.reduce(2) { $0 + $1.value.utf8String().count + 1 })

        var mergedError: String? = nil
        for (i, it) in sorted.enumerated() {
            if i > 0 { out.append(",") }
            out.append(it.value.utf8String())
            mergedError = joinErrors(mergedError, it.error)
        }
        out.append("]")

        return JsonCarrier(value: RawJSON(utf8String: out), index: 0, error: mergedError)
    }
}

// MARK: - JsonGenericCarrier<T> (mirrors Go `textual.JsonGenericCarrier[T]`)

/// JsonGenericCarrier is a typed JSON carrier mirroring Go's `textual.JsonGenericCarrier[T]`.
///
/// Notes:
///   - `utf8String()` returns the JSON encoding of the carrier itself
///     (including `value`, `index`, `error`), mirroring the Go implementation.
///   - `fromUTF8String(_:)` attempts to decode the carrier from its JSON representation.
///     On decode failure, it returns a carrier whose `error` contains the decoding error
///     and whose `value` is `T()` (a Swift analogue of Go's "zero value").
public struct JsonGenericCarrier<T: Codable & ZeroInitializable>: Codable, Equatable, Carrier {
    public var value: T
    public var index: Int
    public var error: String?

    public init(value: T, index: Int = 0, error: String? = nil) {
        self.value = value
        self.index = index
        self.error = normalizeError(error)
    }

    public func utf8String() -> UTF8String {
        do {
            let data = try JSONEncoder().encode(self)
            return String(decoding: data, as: UTF8.self)
        } catch {
            return error.localizedDescription
        }
    }

    public func fromUTF8String(_ s: UTF8String) -> JsonGenericCarrier<T> {
        do {
            let decoded = try JSONDecoder().decode(JsonGenericCarrier<T>.self, from: Data(s.utf8))
            return decoded
        } catch {
            return JsonGenericCarrier<T>(value: T(), index: 0, error: error.localizedDescription)
        }
    }

    public func withIndex(_ index: Int) -> JsonGenericCarrier<T> {
        var copy = self
        copy.index = index
        return copy
    }

    public func getIndex() -> Int { index }

    public func withError(_ error: String?) -> JsonGenericCarrier<T> {
        var copy = self
        copy.error = joinErrors(copy.error, error)
        return copy
    }

    public func getError() -> String? { error }
}

// MARK: - Parcel (mirrors Go `textual.Parcel`)

public struct Fragment: Codable, Equatable {
    public var transformed: UTF8String
    public var pos: Int
    public var len: Int
    public var confidence: Double
    public var variant: Int

    public init(
        transformed: UTF8String,
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

public struct RawText: Codable, Equatable {
    public var text: UTF8String
    public var pos: Int
    public var len: Int

    public init(text: UTF8String, pos: Int, len: Int) {
        self.text = text
        self.pos = pos
        self.len = len
    }
}

/// Parcel is a rich carrier designed for partial transformations.
///
/// It keeps the original input (`text`) and a set of transformed spans (`fragments`).
/// Each fragment references a Unicode-scalar-based range within `text` using (pos, len).
///
/// Positions and lengths are expressed in **Unicode scalars**, which maps to Go’s
/// `rune` indexing for UTF-8 strings.
public struct Parcel: Codable, Equatable, Carrier {
    public var index: Int
    public var text: UTF8String
    public var fragments: [Fragment]
    public var error: String?

    public init(index: Int = -1, text: UTF8String, fragments: [Fragment] = [], error: String? = nil) {
        self.index = index
        self.text = text
        self.fragments = fragments
        self.error = normalizeError(error)
    }

    // MARK: - Carrier

    public func utf8String() -> UTF8String { render() }

    public func fromUTF8String(_ s: UTF8String) -> Parcel {
        return Parcel(index: -1, text: s, fragments: [], error: nil)
    }

    public func withIndex(_ index: Int) -> Parcel {
        var copy = self
        copy.index = index
        return copy
    }

    public func getIndex() -> Int { index }

    public func withError(_ error: String?) -> Parcel {
        var copy = self
        copy.error = joinErrors(copy.error, error)
        return copy
    }

    public func getError() -> String? { error }

    // MARK: - RawTexts (mirrors Go Parcel.RawTexts)

    /// rawTexts computes the non-transformed segments of the original text.
    ///
    /// Behaviour mirrors the Go implementation:
    ///   - fragments are copied and sorted by (pos, len)
    ///   - overlaps are merged by advancing a cursor to the furthest end seen
    ///   - out-of-range bounds are clamped so this never traps
    public func rawTexts() -> [RawText] {
        var raw: [RawText] = []

        let scalars = Array(text.unicodeScalars)
        let textLen = scalars.count
        guard textLen > 0 else { return raw }

        guard !fragments.isEmpty else {
            raw.append(RawText(text: text, pos: 0, len: textLen))
            return raw
        }

        var sortedFragments = fragments
        sortedFragments.sort { a, b in
            if a.pos == b.pos { return a.len < b.len }
            return a.pos < b.pos
        }

        var cursor = 0

        for fragment in sortedFragments {
            guard fragment.len > 0 else { continue }

            var start = fragment.pos
            var end = fragment.pos + fragment.len

            if start < 0 { start = 0 }
            if start >= textLen { continue }
            if end > textLen { end = textLen }

            if cursor < start {
                let slice = scalars[cursor..<start]
                raw.append(RawText(text: stringFromScalars(slice), pos: cursor, len: start - cursor))
            }

            if cursor < end { cursor = end }
        }

        if cursor < textLen {
            let slice = scalars[cursor..<textLen]
            raw.append(RawText(text: stringFromScalars(slice), pos: cursor, len: textLen - cursor))
        }

        return raw
    }

    // MARK: - Render / UTF8String (mirrors Go Parcel.UTF8String)

    /// render reconstructs a plain string by interleaving transformed fragments
    /// and raw text segments.
    ///
    /// It mirrors the Go implementation:
    ///   - only the first fragment for a given pos is rendered (variants are ignored)
    ///   - segments are sorted by pos and concatenated
    public func render() -> UTF8String {
        struct Segment {
            let pos: Int
            let text: UTF8String
        }

        var segments: [Segment] = []
        segments.reserveCapacity(fragments.count + rawTexts().count)

        // First: fragments (dedupe by pos, preserving order of first occurrence).
        var lastFragPos = -1
        for fragment in fragments {
            if fragment.pos != lastFragPos {
                segments.append(Segment(pos: fragment.pos, text: fragment.transformed))
                lastFragPos = fragment.pos
            }
        }

        // Then: raw segments.
        for raw in rawTexts() {
            segments.append(Segment(pos: raw.pos, text: raw.text))
        }

        segments.sort { $0.pos < $1.pos }

        var out = ""
        out.reserveCapacity(text.count)

        for seg in segments {
            out.append(seg.text)
        }
        return out
    }

    // MARK: - Aggregation helper (optional)

    /// aggregate concatenates multiple parcels into a single parcel while preserving fragment coordinates.
    ///
    /// The input parcels are stably sorted by index (negative indices are treated as "unset" and come last).
    /// Fragment positions are offset as texts are concatenated (in Unicode scalars).
    public func aggregate(_ parcels: [Parcel]) -> Parcel {
        return Parcel.aggregate(parcels)
    }

    public static func aggregate(_ parcels: [Parcel]) -> Parcel {
        let sorted = stableSorted(parcels) { orderingKey($0.index) }

        var outText = ""
        outText.reserveCapacity(sorted.reduce(0) { $0 + $1.text.count })

        var outFragments: [Fragment] = []
        outFragments.reserveCapacity(sorted.reduce(0) { $0 + $1.fragments.count })

        var offset = 0
        var mergedError: String? = nil

        for p in sorted {
            mergedError = joinErrors(mergedError, p.error)

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

/// input is a convenience factory mirroring the JS helper `input(text)`.
@discardableResult
public func input(_ text: UTF8String) -> Parcel {
    return Parcel(index: -1, text: text, fragments: [], error: nil)
}

// MARK: - Streaming framing helpers (Swift equivalents of Go split funcs)

public typealias ScanResult = (advance: Int, token: Data?, error: Error?)

// MARK: scanLines (mirrors Go ScanLines)

/// scanLines is a framing helper that returns each line including any trailing '\n'.
///
/// It is equivalent to the Go `ScanLines` split func in this repository
/// (not bufio.ScanLines).
public func scanLines(_ data: Data, atEOF: Bool) -> ScanResult {
    if atEOF && data.isEmpty {
        return (advance: 0, token: nil, error: nil)
    }

    if let nl = data.firstIndex(of: 0x0A) {
        let end = nl + 1
        return (advance: end, token: data.subdata(in: 0..<end), error: nil)
    }

    if atEOF {
        return (advance: data.count, token: data, error: nil)
    }

    return (advance: 0, token: nil, error: nil)
}

/// Convenience overload that scans a UTF-8 string buffer.
///
/// - Note: `advance` is expressed in **UTF-8 bytes**, not Swift `String.Index`.
public func scanLines(_ text: UTF8String, atEOF: Bool) -> (advance: Int, token: UTF8String?, error: Error?) {
    let res = scanLines(Data(text.utf8), atEOF: atEOF)
    let tokenText = res.token.map { String(decoding: $0, as: UTF8.self) }
    return (advance: res.advance, token: tokenText, error: res.error)
}

// MARK: scanExpression (mirrors Go ScanExpression)

private func decodeUTF8Scalar(_ data: Data, from index: Int) -> (scalar: UnicodeScalar, size: Int, valid: Bool) {
    let count = data.count
    guard index < count else {
        return (scalar: UnicodeScalar(0xFFFD)!, size: 1, valid: false)
    }

    let b0 = data[index]

    // ASCII fast path.
    if b0 < 0x80 {
        return (scalar: UnicodeScalar(b0), size: 1, valid: true)
    }

    // Continuation byte or invalid leading byte.
    if b0 < 0xC0 {
        return (scalar: UnicodeScalar(0xFFFD)!, size: 1, valid: false)
    }

    // 2-byte sequence.
    if b0 < 0xE0 {
        guard index + 1 < count else { return (UnicodeScalar(0xFFFD)!, 1, false) }
        let b1 = data[index + 1]
        guard (b1 & 0xC0) == 0x80 else { return (UnicodeScalar(0xFFFD)!, 1, false) }
        let scalarValue = UInt32(b0 & 0x1F) << 6 | UInt32(b1 & 0x3F)
        guard scalarValue >= 0x80 else { return (UnicodeScalar(0xFFFD)!, 1, false) }
        if let s = UnicodeScalar(scalarValue) {
            return (s, 2, true)
        }
        return (UnicodeScalar(0xFFFD)!, 1, false)
    }

    // 3-byte sequence.
    if b0 < 0xF0 {
        guard index + 2 < count else { return (UnicodeScalar(0xFFFD)!, 1, false) }
        let b1 = data[index + 1]
        let b2 = data[index + 2]
        guard (b1 & 0xC0) == 0x80, (b2 & 0xC0) == 0x80 else { return (UnicodeScalar(0xFFFD)!, 1, false) }
        let scalarValue = UInt32(b0 & 0x0F) << 12 | UInt32(b1 & 0x3F) << 6 | UInt32(b2 & 0x3F)
        // Reject overlong and surrogate range.
        guard scalarValue >= 0x800, !(0xD800...0xDFFF).contains(Int(scalarValue)) else {
            return (UnicodeScalar(0xFFFD)!, 1, false)
        }
        if let s = UnicodeScalar(scalarValue) {
            return (s, 3, true)
        }
        return (UnicodeScalar(0xFFFD)!, 1, false)
    }

    // 4-byte sequence.
    if b0 < 0xF5 {
        guard index + 3 < count else { return (UnicodeScalar(0xFFFD)!, 1, false) }
        let b1 = data[index + 1]
        let b2 = data[index + 2]
        let b3 = data[index + 3]
        guard (b1 & 0xC0) == 0x80, (b2 & 0xC0) == 0x80, (b3 & 0xC0) == 0x80 else {
            return (UnicodeScalar(0xFFFD)!, 1, false)
        }
        let scalarValue = UInt32(b0 & 0x07) << 18 | UInt32(b1 & 0x3F) << 12 | UInt32(b2 & 0x3F) << 6 | UInt32(b3 & 0x3F)
        guard scalarValue >= 0x10000, scalarValue <= 0x10FFFF else {
            return (UnicodeScalar(0xFFFD)!, 1, false)
        }
        if let s = UnicodeScalar(scalarValue) {
            return (s, 4, true)
        }
        return (UnicodeScalar(0xFFFD)!, 1, false)
    }

    // Invalid.
    return (UnicodeScalar(0xFFFD)!, 1, false)
}

/// scanExpression tokenizes a buffer into "expressions" centered around words.
///
/// Each token has the shape:
///   [optional leading whitespace][non-empty run of non-whitespace][optional trailing whitespace]
///
/// This mirrors the Go `ScanExpression` split func.
public func scanExpression(_ data: Data, atEOF: Bool) -> ScanResult {
    if atEOF && data.isEmpty {
        return (advance: 0, token: nil, error: nil)
    }

    // Find the first non-space rune, keeping leading whitespace in the token.
    var firstNonSpace: Int? = nil
    var i = 0
    while i < data.count {
        let d = decodeUTF8Scalar(data, from: i)
        if !d.valid && d.size == 1 {
            firstNonSpace = i
            break
        }
        if !d.scalar.properties.isWhitespace {
            firstNonSpace = i
            break
        }
        i += d.size
    }

    guard let first = firstNonSpace else {
        // Buffer currently only contains whitespace.
        if atEOF {
            return (advance: data.count, token: data, error: nil)
        }
        return (advance: 0, token: nil, error: nil)
    }

    // Find the end of the non-space core.
    var endNon = first
    while endNon < data.count {
        let d = decodeUTF8Scalar(data, from: endNon)
        if !d.valid && d.size == 1 {
            endNon += d.size
            continue
        }
        if d.scalar.properties.isWhitespace {
            break
        }
        endNon += d.size
    }

    if endNon == data.count {
        if atEOF {
            return (advance: data.count, token: data, error: nil)
        }
        return (advance: 0, token: nil, error: nil)
    }

    // Extend through trailing whitespace.
    var end = endNon
    while end < data.count {
        let d = decodeUTF8Scalar(data, from: end)
        if !d.valid && d.size == 1 {
            break
        }
        if !d.scalar.properties.isWhitespace {
            break
        }
        end += d.size
    }

    return (advance: end, token: data.subdata(in: 0..<end), error: nil)
}

public func scanExpression(_ text: UTF8String, atEOF: Bool) -> (advance: Int, token: UTF8String?, error: Error?) {
    let res = scanExpression(Data(text.utf8), atEOF: atEOF)
    let tokenText = res.token.map { String(decoding: $0, as: UTF8.self) }
    return (advance: res.advance, token: tokenText, error: res.error)
}

// MARK: scanJSON (mirrors Go ScanJSON)

public enum JSONScanError: Error, LocalizedError, Equatable {
    case unexpectedEOF
    case unexpectedClosing(byte: UInt8, index: Int)
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
            if byte >= 0x20 && byte <= 0x7E {
                return "'" + String(UnicodeScalar(byte)) + "'"
            }
            return "0x" + String(byte, radix: 16, uppercase: true)
        }
    }
}

/// scanJSON tokenizes a buffer into a single top-level JSON value (object or array).
///
/// Mirrors the Go `ScanJSON` split func behaviour:
///   - Leading bytes before the first '{' or '[' are ignored (consumed).
///   - Strings and escapes are handled so braces inside strings do not affect nesting.
///   - When a complete value is present, `advance` consumes *everything up to the end*
///     (including any ignored leading bytes) and `token` is the JSON value slice.
public func scanJSON(_ data: Data, atEOF: Bool) -> ScanResult {
    if atEOF && data.isEmpty {
        return (advance: 0, token: nil, error: nil)
    }

    // Find the first '{' or '['. Everything before it is ignored.
    var start: Int? = nil
    for i in 0..<data.count {
        let b = data[i]
        if b == 0x7B || b == 0x5B {
            start = i
            break
        }
    }

    guard let startIndex = start else {
        // No opening delimiter in the current buffer; consume it all (noise).
        return (advance: data.count, token: nil, error: nil)
    }

    // Stack tracks nesting.
    var stack: [UInt8] = []
    stack.reserveCapacity(8)
    stack.append(data[startIndex])

    var inString = false
    var escaped = false

    var i = startIndex + 1
    while i < data.count {
        let b = data[i]

        if inString {
            if escaped {
                escaped = false
                i += 1
                continue
            }
            if b == 0x5C { // backslash
                escaped = true
                i += 1
                continue
            }
            if b == 0x22 { // quote
                inString = false
            }
            i += 1
            continue
        }

        // Outside of strings.
        if b == 0x22 {
            inString = true
            i += 1
            continue
        }

        if b == 0x7B || b == 0x5B {
            stack.append(b)
            i += 1
            continue
        }

        if b == 0x7D || b == 0x5D {
            guard let top = stack.last else {
                return (advance: 0, token: nil, error: JSONScanError.unexpectedClosing(byte: b, index: i))
            }

            let matches = (b == 0x7D && top == 0x7B) || (b == 0x5D && top == 0x5B)
            guard matches else {
                return (advance: 0, token: nil, error: JSONScanError.mismatchedClosing(byte: b, expectedOpen: top, index: i))
            }

            stack.removeLast()
            if stack.isEmpty {
                let end = i + 1
                return (advance: end, token: data.subdata(in: startIndex..<end), error: nil)
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
    // If we had to skip leading noise, consume it now.
    if startIndex > 0 {
        return (advance: startIndex, token: nil, error: nil)
    }
    return (advance: 0, token: nil, error: nil)
}

public func scanJSON(_ text: UTF8String, atEOF: Bool) -> (advance: Int, token: UTF8String?, error: Error?) {
    let res = scanJSON(Data(text.utf8), atEOF: atEOF)
    let tokenText = res.token.map { String(decoding: $0, as: UTF8.self) }
    return (advance: res.advance, token: tokenText, error: res.error)
}

// MARK: scanCSV (mirrors Go ScanCSV)

public enum CSVScanError: Error, LocalizedError, Equatable {
    case unexpectedEOF

    public var errorDescription: String? {
        switch self {
        case .unexpectedEOF:
            return "Unexpected EOF while scanning CSV (unterminated quoted field)"
        }
    }
}

/// scanCSV tokenizes an input buffer into CSV records.
///
/// Mirrors the Go `ScanCSV` framing rules:
///   - Records end on '\n', '\r\n', or '\r' when the delimiter occurs OUTSIDE quotes.
///   - Quotes use standard CSV escaping rules: '""' represents an escaped quote.
///   - Returned token does NOT include the trailing record separator.
///   - If `atEOF` is true while inside quotes, returns unexpectedEOF.
public func scanCSV(_ data: Data, atEOF: Bool) -> ScanResult {
    if atEOF && data.isEmpty {
        return (advance: 0, token: nil, error: nil)
    }

    var inQuotes = false
    var i = 0

    while i < data.count {
        let b = data[i]

        if b == 0x22 { // '"'
            if inQuotes {
                // Inside quotes, a doubled quote ("") is an escaped quote.
                if i + 1 < data.count && data[i + 1] == 0x22 {
                    i += 2
                    continue
                }
                // Otherwise this closes the quoted field.
                inQuotes = false
                i += 1
                continue
            } else {
                // Opening quote.
                inQuotes = true
                i += 1
                continue
            }
        }

        if !inQuotes {
            switch b {
            case 0x0A: // '\n'
                var end = i
                if end > 0 && data[end - 1] == 0x0D { // trim preceding '\r'
                    end -= 1
                }
                return (advance: i + 1, token: data.subdata(in: 0..<end), error: nil)

            case 0x0D: // '\r'
                let end = i
                var adv = i + 1
                if i + 1 < data.count && data[i + 1] == 0x0A { // '\n'
                    adv = i + 2
                }
                return (advance: adv, token: data.subdata(in: 0..<end), error: nil)

            default:
                break
            }
        }

        i += 1
    }

    if atEOF {
        if inQuotes {
            return (advance: 0, token: nil, error: CSVScanError.unexpectedEOF)
        }
        return (advance: data.count, token: data, error: nil)
    }

    return (advance: 0, token: nil, error: nil)
}

public func scanCSV(_ text: UTF8String, atEOF: Bool) -> (advance: Int, token: UTF8String?, error: Error?) {
    let res = scanCSV(Data(text.utf8), atEOF: atEOF)
    let tokenText = res.token.map { String(decoding: $0, as: UTF8.self) }
    return (advance: res.advance, token: tokenText, error: res.error)
}

// MARK: scanXML (mirrors Go ScanXML)

public enum XMLScanError: Error, LocalizedError, Equatable {
    case unexpectedEOF
    case unexpectedClosingTag(name: String, index: Int)
    case mismatchedClosingTag(name: String, expectedOpen: String, index: Int)

    public var errorDescription: String? {
        switch self {
        case .unexpectedEOF:
            return "Unexpected EOF while scanning XML"
        case .unexpectedClosingTag(let name, let index):
            return "Unexpected closing tag </\(name)> at byte \(index)"
        case .mismatchedClosingTag(let name, let expectedOpen, let index):
            return "Mismatched closing tag </\(name)> for <\(expectedOpen)> at byte \(index)"
        }
    }
}

private let xmlCommentOpen = Data("<!--".utf8)
private let xmlCommentClose = Data("-->".utf8)

private let xmlCDATAOpen = Data("<![CDATA[".utf8)
private let xmlCDATAClose = Data("]]>".utf8)

private let xmlPIClose = Data("?>".utf8)

private func hasPrefixBytes(_ data: Data, at index: Int, prefix: Data) -> Bool {
    guard index >= 0, index + prefix.count <= data.count else { return false }
    return data.subdata(in: index..<(index + prefix.count)) == prefix
}

/// indexAfter searches for `needle` in data starting at offset `from`.
/// It returns the index immediately AFTER the needle when found.
private func indexAfter(_ data: Data, from: Int, needle: Data) -> Int? {
    let start = max(0, min(from, data.count))
    let range = start..<data.count
    guard let found = data.range(of: needle, options: [], in: range) else { return nil }
    return found.upperBound
}

private func findFirstStartElement(_ data: Data) -> Int? {
    guard data.count >= 2 else { return nil }
    var i = 0
    while i < data.count {
        if data[i] == 0x3C { // '<'
            if i + 1 >= data.count { return nil }
            let n = data[i + 1]
            if n == 0x2F || n == 0x21 || n == 0x3F { // '/', '!', '?'
                i += 1
                continue
            }
            if isXMLNameStart(n) {
                return i
            }
        }
        i += 1
    }
    return nil
}

private func isXMLNameStart(_ b: UInt8) -> Bool {
    return (b >= 0x41 && b <= 0x5A) || // A-Z
           (b >= 0x61 && b <= 0x7A) || // a-z
           b == 0x5F || b == 0x3A     // '_' or ':'
}

private func isXMLNameChar(_ b: UInt8) -> Bool {
    return isXMLNameStart(b) ||
           (b >= 0x30 && b <= 0x39) || // 0-9
           b == 0x2D || b == 0x2E      // '-' or '.'
}

private func scanName(_ data: Data, from: Int) -> (name: String, next: Int)? {
    guard from < data.count else { return nil }
    var i = from
    while i < data.count && isXMLNameChar(data[i]) {
        i += 1
    }
    guard i > from else { return nil }
    let nameData = data.subdata(in: from..<i)
    return (String(decoding: nameData, as: UTF8.self), i)
}

private func scanTagClose(_ data: Data, from: Int) -> Int? {
    guard from < data.count else { return nil }
    for i in from..<data.count {
        if data[i] == 0x3E { // '>'
            return i
        }
    }
    return nil
}

private func isXMLWhitespace(_ b: UInt8) -> Bool {
    return b == 0x20 || b == 0x09 || b == 0x0A || b == 0x0D // space, tab, \n, \r
}

private func scanStartTagClose(_ data: Data, from: Int) -> (closeIdx: Int, selfClosing: Bool)? {
    var quote: UInt8 = 0 // 0, '\'' or '"'
    var i = from
    while i < data.count {
        let b = data[i]

        if quote != 0 {
            if b == quote { quote = 0 }
            i += 1
            continue
        }

        if b == 0x22 || b == 0x27 { // '"' or '\''
            quote = b
            i += 1
            continue
        }

        if b == 0x3E { // '>'
            // Determine whether this is "/>" (ignoring trailing whitespace).
            var j = i - 1
            while j >= from && isXMLWhitespace(data[j]) {
                j -= 1
            }
            if j >= from && data[j] == 0x2F { // '/'
                return (closeIdx: i, selfClosing: true)
            }
            return (closeIdx: i, selfClosing: false)
        }

        i += 1
    }
    return nil
}

private func scanDirectiveEnd(_ data: Data, from: Int) -> Int? {
    var quote: UInt8 = 0
    var bracketDepth = 0

    var i = from
    while i < data.count {
        let b = data[i]

        if quote != 0 {
            if b == quote { quote = 0 }
            i += 1
            continue
        }

        if b == 0x22 || b == 0x27 { // '"' or '\''
            quote = b
            i += 1
            continue
        }

        switch b {
        case 0x5B: // '['
            bracketDepth += 1
        case 0x5D: // ']'
            if bracketDepth > 0 { bracketDepth -= 1 }
        case 0x3E: // '>'
            if bracketDepth == 0 { return i + 1 }
        default:
            break
        }

        i += 1
    }
    return nil
}

/// scanXML tokenizes an input stream into top-level XML elements (one complete element per token).
///
/// Mirrors the Go `ScanXML` framing helper.
public func scanXML(_ data: Data, atEOF: Bool) -> ScanResult {
    if atEOF && data.isEmpty {
        return (advance: 0, token: nil, error: nil)
    }

    guard let start = findFirstStartElement(data) else {
        // No element start found: consume noise.
        return (advance: data.count, token: nil, error: nil)
    }

    var i = start
    var stack: [String] = []
    stack.reserveCapacity(8)

    while i < data.count {
        if data[i] != 0x3C { // '<'
            i += 1
            continue
        }

        if i + 1 >= data.count {
            if atEOF { return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF) }
            if start > 0 { return (advance: start, token: nil, error: nil) }
            return (advance: 0, token: nil, error: nil)
        }

        // 1) Comments: <!-- ... -->
        if data[i + 1] == 0x21, hasPrefixBytes(data, at: i, prefix: xmlCommentOpen) {
            guard let end = indexAfter(data, from: i + xmlCommentOpen.count, needle: xmlCommentClose) else {
                if atEOF { return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF) }
                if start > 0 { return (advance: start, token: nil, error: nil) }
                return (advance: 0, token: nil, error: nil)
            }
            i = end
            continue
        }

        // 2) CDATA: <![CDATA[ ... ]]>
        if data[i + 1] == 0x21, hasPrefixBytes(data, at: i, prefix: xmlCDATAOpen) {
            guard let end = indexAfter(data, from: i + xmlCDATAOpen.count, needle: xmlCDATAClose) else {
                if atEOF { return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF) }
                if start > 0 { return (advance: start, token: nil, error: nil) }
                return (advance: 0, token: nil, error: nil)
            }
            i = end
            continue
        }

        // 3) Processing instruction: <? ... ?>
        if data[i + 1] == 0x3F { // '?'
            guard let end = indexAfter(data, from: i + 2, needle: xmlPIClose) else {
                if atEOF { return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF) }
                if start > 0 { return (advance: start, token: nil, error: nil) }
                return (advance: 0, token: nil, error: nil)
            }
            i = end
            continue
        }

        // 4) Directives / doctype / declarations: <! ... >
        if data[i + 1] == 0x21 { // '!'
            guard let end = scanDirectiveEnd(data, from: i + 2) else {
                if atEOF { return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF) }
                if start > 0 { return (advance: start, token: nil, error: nil) }
                return (advance: 0, token: nil, error: nil)
            }
            i = end
            continue
        }

        // 5) End tag: </name>
        if data[i + 1] == 0x2F { // '/'
            guard let parsed = scanName(data, from: i + 2) else {
                if atEOF { return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF) }
                if start > 0 { return (advance: start, token: nil, error: nil) }
                return (advance: 0, token: nil, error: nil)
            }
            let name = parsed.name
            let nameEnd = parsed.next

            guard let closeIdx = scanTagClose(data, from: nameEnd) else {
                if atEOF { return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF) }
                if start > 0 { return (advance: start, token: nil, error: nil) }
                return (advance: 0, token: nil, error: nil)
            }

            guard let top = stack.last else {
                return (advance: 0, token: nil, error: XMLScanError.unexpectedClosingTag(name: name, index: i))
            }
            guard top == name else {
                return (advance: 0, token: nil, error: XMLScanError.mismatchedClosingTag(name: name, expectedOpen: top, index: i))
            }

            stack.removeLast()
            i = closeIdx + 1

            if stack.isEmpty {
                return (advance: i, token: data.subdata(in: start..<i), error: nil)
            }
            continue
        }

        // 6) Start tag: <name ...> or <name .../>
        if isXMLNameStart(data[i + 1]) {
            guard let parsed = scanName(data, from: i + 1) else {
                if atEOF { return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF) }
                if start > 0 { return (advance: start, token: nil, error: nil) }
                return (advance: 0, token: nil, error: nil)
            }
            let name = parsed.name
            let nameEnd = parsed.next

            guard let close = scanStartTagClose(data, from: nameEnd) else {
                if atEOF { return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF) }
                if start > 0 { return (advance: start, token: nil, error: nil) }
                return (advance: 0, token: nil, error: nil)
            }

            if close.selfClosing {
                // Root self-closing element: complete token immediately.
                if stack.isEmpty {
                    i = close.closeIdx + 1
                    return (advance: i, token: data.subdata(in: start..<i), error: nil)
                }
                // Nested self-closing element: no stack change.
                i = close.closeIdx + 1
                continue
            }

            // Regular start element: push to stack.
            stack.append(name)
            i = close.closeIdx + 1
            continue
        }

        // Unrecognized '<' sequence: move forward to avoid infinite loops.
        i += 1
    }

    // Buffer ended before we closed the root element.
    if atEOF {
        return (advance: 0, token: nil, error: XMLScanError.unexpectedEOF)
    }
    if start > 0 {
        return (advance: start, token: nil, error: nil)
    }
    return (advance: 0, token: nil, error: nil)
}

public func scanXML(_ text: UTF8String, atEOF: Bool) -> (advance: Int, token: UTF8String?, error: Error?) {
    let res = scanXML(Data(text.utf8), atEOF: atEOF)
    let tokenText = res.token.map { String(decoding: $0, as: UTF8.self) }
    return (advance: res.advance, token: tokenText, error: res.error)
}

// MARK: - Casting helpers (Swift equivalents of carrier_facilities.go helpers)

public enum CarrierCastError: Error, LocalizedError, Equatable {
    case upstreamError(index: Int, message: String)
    case emptyValue(index: Int, kind: String)
    case decodingError(index: Int, message: String)
    case unsupportedType(index: Int, kind: String)

    public var errorDescription: String? {
        switch self {
        case .upstreamError(let index, let message):
            return index == 0 ? message : "\(kindPrefix(index)): \(message)"
        case .emptyValue(let index, let kind):
            let msg = "empty \(kind) value"
            return index == 0 ? msg : "\(kindPrefix(index)): \(msg)"
        case .decodingError(let index, let message):
            return index == 0 ? message : "\(kindPrefix(index)): \(message)"
        case .unsupportedType(let index, let kind):
            let msg = "unsupported cast target for \(kind)"
            return index == 0 ? msg : "\(kindPrefix(index)): \(msg)"
        }
    }

    private static func kindPrefix(_ index: Int) -> String {
        return "index \(index)"
    }
}

/// CastJson attempts to decode the raw JSON value carried by JsonCarrier into T.
///
/// This mirrors the Go helper `CastJson`:
///   - If the carrier already has an error, decoding is skipped and the error is returned.
///   - If the carrier value is empty, an error is returned.
///   - Otherwise the JSON value is decoded into T using JSONDecoder.
public func CastJson<T: Decodable>(_ j: JsonCarrier, as _: T.Type = T.self) throws -> T {
    if let err = normalizeError(j.error) {
        throw CarrierCastError.upstreamError(index: j.index, message: err)
    }
    if j.value.bytes.isEmpty {
        throw CarrierCastError.emptyValue(index: j.index, kind: "JsonCarrier")
    }

    // Fast paths.
    if T.self == RawJSON.self, let v = j.value as? T {
        return v
    }
    if T.self == Data.self, let v = j.value.bytes as? T {
        return v
    }
    if T.self == JsonCarrier.self, let v = JsonCarrier(value: j.value, index: j.index, error: nil) as? T {
        return v
    }

    do {
        return try JSONDecoder().decode(T.self, from: j.value.bytes)
    } catch {
        throw CarrierCastError.decodingError(index: j.index, message: error.localizedDescription)
    }
}

/// CastCsvRecord parses a single CSV record into an array of fields.
///
/// This is a lightweight Swift analogue of Go's `CastCsvRecord` + `encoding/csv`
/// for the common case where the carrier holds exactly one record (as produced by scanCSV).
public func CastCsvRecord(_ c: CsvCarrier) throws -> [String] {
    if let err = normalizeError(c.error) {
        throw CarrierCastError.upstreamError(index: c.index, message: err)
    }
    if c.value.isEmpty {
        throw CarrierCastError.emptyValue(index: c.index, kind: "CsvCarrier")
    }

    return try parseCSVRecord(c.value)
}

/// CastXml provides minimal "raw" casts for XmlCarrier.
///
/// Swift does not ship a generic XML decoder equivalent to Go's `encoding/xml.Unmarshal`.
/// This helper supports:
///   - String: returns the XML fragment as-is
///   - Data: returns UTF-8 bytes of the fragment
///   - XmlCarrier: returns the carrier with its error cleared
public func CastXml<T>(_ x: XmlCarrier, as _: T.Type = T.self) throws -> T {
    if let err = normalizeError(x.error) {
        throw CarrierCastError.upstreamError(index: x.index, message: err)
    }
    if x.value.isEmpty {
        throw CarrierCastError.emptyValue(index: x.index, kind: "XmlCarrier")
    }

    if T.self == String.self, let v = x.value as? T {
        return v
    }
    if T.self == Data.self, let v = Data(x.value.utf8) as? T {
        return v
    }
    if T.self == XmlCarrier.self, let v = XmlCarrier(value: x.value, index: x.index, error: nil) as? T {
        return v
    }

    throw CarrierCastError.unsupportedType(index: x.index, kind: "XmlCarrier")
}

private enum CSVParseError: Error, LocalizedError, Equatable {
    case unexpectedEOF

    var errorDescription: String? {
        switch self {
        case .unexpectedEOF:
            return "Unexpected EOF while parsing CSV record (unterminated quoted field)"
        }
    }
}

/// parseCSVRecord parses a single CSV record using the standard CSV escaping rules:
///   - fields separated by commas
///   - quoted fields start with '"'
///   - inside quoted fields, '""' is an escaped quote
///
/// This is intentionally small and designed for the output of `scanCSV`.
private func parseCSVRecord(_ record: String) throws -> [String] {
    var fields: [String] = []
    fields.reserveCapacity(8)

    var field = ""
    field.reserveCapacity(record.count)

    var inQuotes = false

    var i = record.startIndex
    while i < record.endIndex {
        let ch = record[i]

        if inQuotes {
            if ch == "\"" {
                let next = record.index(after: i)
                if next < record.endIndex, record[next] == "\"" {
                    // Escaped quote.
                    field.append("\"")
                    i = record.index(after: next)
                    continue
                }
                // Closing quote.
                inQuotes = false
                i = next
                continue
            }

            field.append(ch)
            i = record.index(after: i)
            continue
        }

        // Not in quotes.
        if ch == "\"" && field.isEmpty {
            // Only treat quotes as opening when they start the field, matching Go's encoding/csv expectations.
            inQuotes = true
            i = record.index(after: i)
            continue
        }

        if ch == "," {
            fields.append(field)
            field = ""
            i = record.index(after: i)
            continue
        }

        field.append(ch)
        i = record.index(after: i)
    }

    if inQuotes {
        throw CSVParseError.unexpectedEOF
    }

    fields.append(field)
    return fields
}

// MARK: - Encoding catalogue (mirrors Go encoding.go)

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

    /// encodingName mirrors Go's `EncodingName()` method (canonical string).
    public var encodingName: String {
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

    /// Backward-friendly alias.
    public var canonicalName: String { encodingName }

    /// nameToEncoding mirrors Go's `nameToEncoding` map (case-insensitive lookup).
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

    public static func parse(_ name: String) throws -> EncodingID {
        let trimmed = name.trimmingCharacters(in: .whitespacesAndNewlines)
        let key = trimmed.lowercased()
        if let id = nameToEncoding[key] {
            return id
        }
        throw EncodingError.unknownEncoding(trimmed)
    }
}

public enum EncodingError: Error, LocalizedError, Equatable {
    case unknownEncoding(String)

    public var errorDescription: String? {
        switch self {
        case .unknownEncoding(let value):
            return "Unknown encoding: \(value)"
        }
    }
}
