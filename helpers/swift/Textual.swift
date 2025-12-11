//
// Textual.swift
//
// Lightweight Swift utilities to work with "textual" Result objects in
// client-side code (iOS, macOS, watchOS, tvOS).
//
// This file mirrors the behaviour of the Go "textual" package for:
//
//   - Result.rawTexts()
//   - Result.render()
//
// and exposes the same EncodingID catalogue and name lookup helpers as
// encoding.go.
//
// It is transport-agnostic: it assumes you already receive Result-like JSON
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

/// UTF8String is used for expressivity: all strings are assumed to be UTF-8.
public typealias UTF8String = String

// MARK: - Core data types

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
    public var transformed: UTF8String

    /// First scalar position in the original text.
    public var pos: Int

    /// Length in scalars in the original text.
    public var len: Int

    /// Optional confidence indicator for the transformation.
    public var confidence: Double

    /// Optional variant index when several candidates exist.
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

/// RawText represents an untouched segment of the original text.
///
/// It is computed from a Result and covers every range that is not overlapped
/// by any Fragment (after merging overlapping fragments).
public struct RawText: Codable, Equatable {
    /// The raw text content.
    public var text: UTF8String

    /// First scalar position in the original text.
    public var pos: Int

    /// Length in scalars.
    public var len: Int

    public init(text: UTF8String, pos: Int, len: Int) {
        self.text = text
        self.pos = pos
        self.len = len
    }
}

/// Result is the central value in the textual pipeline.
///
/// It mirrors the Go struct:
///
///   type Result struct {
///     Index     int
///     Text      UTF8String
///     Fragments []Fragment
///     Error     error
///   }
///
/// In Swift:
///   - `text` is a UTF-8 `String`.
///   - `fragments` is an array of Fragment values.
///   - `error` is an optional string; this helper does not interpret it.
public struct Result: Codable, Equatable {
    /// Optional index in a stream of results.
    public var index: Int

    /// Original UTF-8 text.
    public var text: UTF8String

    /// Transformed fragments that reference ranges in `text`.
    public var fragments: [Fragment]

    /// Optional error. Kept as plain text for portability.
    public var error: String?

    public init(
    index: Int = -1,
    text: UTF8String,
    fragments: [Fragment] = [],
    error: String? = nil
    ) {
        self.index = index
        self.text = text
        self.fragments = fragments
        self.error = error
    }
}

// MARK: - Convenience construction

/// Constructs a base Result to be used as a starting point, mirroring the
/// Go helper `Input(text)` and the JS `input(text)` function.
///
/// - Parameter text: Original UTF-8 text.
/// - Returns: A Result with index = -1, no fragments, and no error.
@discardableResult
public func input(_ text: UTF8String) -> Result {
    return Result(index: -1, text: text, fragments: [], error: nil)
}

// MARK: - RawTexts / Render

public extension Result {
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
    /// Rules (matching the Go `Render` method and the JS helper):
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
    func render() -> UTF8String {
        struct Segment {
            let pos: Int
            let text: UTF8String
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