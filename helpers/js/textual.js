

// textual.js
// Lightweight ES6 utilities to work with "textual" Result objects in a browser.
//
// This file mirrors the behavior of the Go "textual" package for:
//   - Result.RawTexts()
//   - Result.Render()
// and exposes an EncodingID map plus helpers similar to encoding.go.
//
// It is transport-agnostic: it assumes you already receive Result-like JSON
// objects (for example, from SSE, WebSocket, or XHR) and helps you manipulate
// them in the browser.
//
// Usage (ES modules):
//   import { Result, input, EncodingID, parseEncoding, encodingName } from './textual-stream-utils.js';
//
//   const res = input('Hello, café');
//   const rawParts = res.rawTexts();
//   const rendered = res.render();
//
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

/**
 * @typedef {string} UTF8String
 * Documentation-only alias: all strings are assumed to be valid UTF-8.
 */

/**
 * Fragment represents a transformed portion of the original text.
 *
 * Positions (pos) and lengths (len) are expressed in Unicode code points,
 * not bytes. This matches the Go implementation which operates in rune space.
 */
export class Fragment {
    /**
     * @param {Object} opts
     * @param {UTF8String} [opts.transformed] - Transformed text (IPA, SAMPA, etc.).
     * @param {number} [opts.pos] - First code-point position in the original text.
     * @param {number} [opts.len] - Length in code points in the original text.
     * @param {number} [opts.confidence] - Optional confidence value.
     * @param {number} [opts.variant] - Optional variant index for multiple candidates.
     */
    constructor({
                    transformed = '',
                    pos = 0,
                    len = 0,
                    confidence = 0,
                    variant = 0
                } = {}) {
        /** @type {UTF8String} */
        this.transformed = String(transformed);
        /** @type {number} */
        this.pos = Number.isFinite(pos) ? Math.trunc(pos) : 0;
        /** @type {number} */
        this.len = Number.isFinite(len) ? Math.trunc(len) : 0;
        /** @type {number} */
        this.confidence = Number.isFinite(confidence) ? confidence : 0;
        /** @type {number} */
        this.variant = Number.isFinite(variant) ? Math.trunc(variant) : 0;
    }
}

/**
 * RawText represents an untouched segment of the original text.
 *
 * It is computed from a Result and covers every range that is not overlapped
 * by any Fragment (after merging overlapping fragments).
 */
export class RawText {
    /**
     * @param {Object} opts
     * @param {UTF8String} [opts.text] - Raw text content.
     * @param {number} [opts.pos] - First code-point position in the original text.
     * @param {number} [opts.len] - Length in code points.
     */
    constructor({ text = '', pos = 0, len = 0 } = {}) {
        /** @type {UTF8String} */
        this.text = String(text);
        /** @type {number} */
        this.pos = Number.isFinite(pos) ? Math.trunc(pos) : 0;
        /** @type {number} */
        this.len = Number.isFinite(len) ? Math.trunc(len) : 0;
    }
}

/**
 * Result is the central value in the textual pipeline.
 *
 * It mirrors the Go struct:
 *   type Result struct {
 *     Index     int
 *     Text      UTF8String
 *     Fragments []Fragment
 *     Error     error
 *   }
 *
 * In JavaScript:
 *   - Text is a UTF-8 string.
 *   - Fragments is an array of Fragment instances.
 *   - Error can be anything (Error instance, string, null, ...).
 */
export class Result {
    /**
     * @param {Object} opts
     * @param {number} [opts.index] - Optional index in a stream.
     * @param {UTF8String} [opts.text] - Original UTF-8 text.
     * @param {Array<Fragment|Object>} [opts.fragments] - Fragments describing transformed regions.
     * @param {*} [opts.error] - Optional error; not interpreted by this module.
     */
    constructor({ index = -1, text = '', fragments = [], error = null } = {}) {
        /** @type {number} */
        this.index = Number.isFinite(index) ? Math.trunc(index) : -1;
        /** @type {UTF8String} */
        this.text = String(text);
        /** @type {Fragment[]} */
        this.fragments = (fragments || []).map((f) =>
            f instanceof Fragment ? f : new Fragment(f)
        );
        /** @type {*} */
        this.error = error;
    }

    /**
     * Constructs a Result from a plain JSON object that follows the same
     * shape as the Go struct when serialized to JSON.
     *
     * @param {Object|Result} json
     * @returns {Result}
     */
    static fromJSON(json) {
        if (json instanceof Result) {
            return json;
        }
        if (!json || typeof json !== 'object') {
            return new Result();
        }
        return new Result({
            index: json.index,
            text: json.text,
            fragments: json.fragments,
            error: json.error || null
        });
    }

    /**
     * Serializes the Result to a plain JSON-friendly object.
     *
     * @returns {Object}
     */
    toJSON() {
        return {
            index: this.index,
            text: this.text,
            fragments: this.fragments.map((f) => ({
                transformed: f.transformed,
                pos: f.pos,
                len: f.len,
                confidence: f.confidence,
                variant: f.variant
            })),
            error: this.error
        };
    }

    /**
     * Computes the non-transformed segments of the original Text.
     *
     * Behaviour is intentionally aligned with the Go implementation:
     *
     *   - If there are no fragments, returns a single RawText covering the whole
     *     text (in code points).
     *   - Fragments are copied, sorted by pos, and treated as a union of ranges.
     *     Overlapping fragments or multiple variants at the same pos are merged
     *     via a moving cursor.
     *   - Zero-length fragments and fragments fully outside the text are ignored.
     *   - Out-of-range fragment bounds are clamped to [0, len(TextInCodePoints)].
     *
     * Positions and lengths are interpreted in code points (Array.from-based),
     * which corresponds to Go runes for UTF-8 strings.
     *
     * @returns {RawText[]} array of RawText segments.
     */
    rawTexts() {
        const raw = [];

        // Work in code-point space (Array.from uses the string iterator, which is
        // based on Unicode code points, not UTF-16 code units).
        const codePoints = Array.from(this.text);
        const textLen = codePoints.length;

        if (textLen === 0) {
            return raw;
        }

        if (!this.fragments || this.fragments.length === 0) {
            raw.push(
                new RawText({
                    text: this.text,
                    pos: 0,
                    len: textLen
                })
            );
            return raw;
        }

        // Copy and sort fragments by start position, tie-breaking by length, in
        // order to compute the union of covered ranges in a single pass.
        const fragments = this.fragments.slice().sort((a, b) => {
            if (a.pos === b.pos) {
                return a.len - b.len;
            }
            return a.pos - b.pos;
        });

        let cursor = 0;

        for (const f of fragments) {
            if (!f || f.len <= 0) {
                continue;
            }

            let start = f.pos;
            let end = f.pos + f.len;

            // Clamp fragment bounds to [0, textLen]
            if (!Number.isFinite(start)) start = 0;
            if (!Number.isFinite(end)) end = start;

            if (start < 0) {
                start = 0;
            }
            if (start >= textLen) {
                // Starts beyond the end of the text: nothing to do.
                continue;
            }
            if (end > textLen) {
                end = textLen;
            }

            // Any gap between cursor and the start of the fragment is raw text.
            if (cursor < start) {
                raw.push(
                    new RawText({
                        text: codePoints.slice(cursor, start).join(''),
                        pos: cursor,
                        len: start - cursor
                    })
                );
            }

            // Advance the cursor to the end of the fragment, never backwards.
            if (cursor < end) {
                cursor = end;
            }
        }

        // Trailing text after the last fragment is also raw.
        if (cursor < textLen) {
            raw.push(
                new RawText({
                    text: codePoints.slice(cursor, textLen).join(''),
                    pos: cursor,
                    len: textLen - cursor
                })
            );
        }

        return raw;
    }

    /**
     * Reconstructs a single output string by merging transformed fragments and
     * raw text segments according to their positions.
     *
     * Rules (matching the Go Render method):
     *   - Both fragments and raw texts reference absolute positions in the
     *     original string.
     *   - All segments (fragments + raw) are collected with their start pos.
     *   - Segments are sorted by pos to restore the original sequence.
     *   - Fragment output uses Fragment.transformed.
     *   - RawText output uses RawText.text.
     *   - No further transformation is applied to the text content.
     *
     * If multiple fragments share the same starting position, only the first
     * one (in the original fragments array) is used, which is consistent with
     * the Go implementation.
     *
     * @returns {UTF8String}
     */
    render() {
        /**
         * @typedef {{pos: number, text: UTF8String}} Segment
         */
        /** @type {Segment[]} */
        const segments = [];

        const rawTexts = this.rawTexts();

        // Convert fragments into segments. Only one fragment per starting
        // position is emitted, mirroring the Go version that uses lastFrag.Pos.
        let lastFragPos = -1;
        if (Array.isArray(this.fragments)) {
            for (const f of this.fragments) {
                if (!f) continue;
                if (f.pos !== lastFragPos) {
                    segments.push({
                        pos: Number.isFinite(f.pos) ? Math.trunc(f.pos) : 0,
                        text: String(f.transformed)
                    });
                    lastFragPos = f.pos;
                }
            }
        }

        // Convert raw text segments into segments as well.
        for (const raw of rawTexts) {
            segments.push({
                pos: Number.isFinite(raw.pos) ? Math.trunc(raw.pos) : 0,
                text: String(raw.text)
            });
        }

        // Sort by position to ensure correct ordering.
        segments.sort((a, b) => a.pos - b.pos);

        // Merge segments into the final output string.
        let out = '';
        for (const seg of segments) {
            out += seg.text;
        }
        return out;
    }
}

/**
 * Convenience factory mirroring the Go Input(...) helper.
 *
 * Creates a base Result with:
 *   - index = -1
 *   - text = given argument
 *   - fragments = []
 *   - error = null
 *
 * @param {UTF8String} text
 * @returns {Result}
 */
export function input(text) {
    return new Result({
        index: -1,
        text: String(text),
        fragments: [],
        error: null
    });
}

/**
 * EncodingID is an enum-like mapping of supported encodings to numeric IDs.
 *
 * The numeric values follow the same ordering as the Go iota-based enum:
 *
 *   0  UTF8
 *   1  UTF16LE
 *   2  UTF16BE
 *   3  UTF16LEBOM
 *   4  UTF16BEBOM
 *   5  ISO8859_1
 *   ...
 *   40 EUCKR
 */
export const EncodingID = Object.freeze({
    UTF8: 0,
    UTF16LE: 1,
    UTF16BE: 2,
    UTF16LEBOM: 3,
    UTF16BEBOM: 4,

    ISO8859_1: 5,
    ISO8859_2: 6,
    ISO8859_3: 7,
    ISO8859_4: 8,
    ISO8859_5: 9,
    ISO8859_6: 10,
    ISO8859_7: 11,
    ISO8859_8: 12,
    ISO8859_9: 13,
    ISO8859_10: 14,
    ISO8859_13: 15,
    ISO8859_14: 16,
    ISO8859_15: 17,
    ISO8859_16: 18,

    KOI8R: 19,
    KOI8U: 20,

    Windows874: 21,
    Windows1250: 22,
    Windows1251: 23,
    Windows1252: 24,
    Windows1253: 25,
    Windows1254: 26,
    Windows1255: 27,
    Windows1256: 28,
    Windows1257: 29,
    Windows1258: 30,

    MacRoman: 31,
    MacCyrillic: 32,

    ShiftJIS: 33,
    EUCJP: 34,
    ISO2022JP: 35,

    GBK: 36,
    HZGB2312: 37,
    GB18030: 38,

    Big5: 39,

    EUCKR: 40
});

/**
 * Canonical encoding names by EncodingID, mirroring EncodingName() in Go.
 *
 * Keys are numeric EncodingID values, values are canonical strings such as
 * "UTF-8", "ISO-8859-1", ...
 */
export const EncodingIdToCanonicalName = Object.freeze({
    [EncodingID.UTF8]: 'UTF-8',
    [EncodingID.UTF16LE]: 'UTF-16LE',
    [EncodingID.UTF16BE]: 'UTF-16BE',
    [EncodingID.UTF16LEBOM]: 'UTF-16LE-BOM',
    [EncodingID.UTF16BEBOM]: 'UTF-16BE-BOM',

    [EncodingID.ISO8859_1]: 'ISO-8859-1',
    [EncodingID.ISO8859_2]: 'ISO-8859-2',
    [EncodingID.ISO8859_3]: 'ISO-8859-3',
    [EncodingID.ISO8859_4]: 'ISO-8859-4',
    [EncodingID.ISO8859_5]: 'ISO-8859-5',
    [EncodingID.ISO8859_6]: 'ISO-8859-6',
    [EncodingID.ISO8859_7]: 'ISO-8859-7',
    [EncodingID.ISO8859_8]: 'ISO-8859-8',
    [EncodingID.ISO8859_9]: 'ISO-8859-9',
    [EncodingID.ISO8859_10]: 'ISO-8859-10',
    [EncodingID.ISO8859_13]: 'ISO-8859-13',
    [EncodingID.ISO8859_14]: 'ISO-8859-14',
    [EncodingID.ISO8859_15]: 'ISO-8859-15',
    [EncodingID.ISO8859_16]: 'ISO-8859-16',

    [EncodingID.KOI8R]: 'KOI8-R',
    [EncodingID.KOI8U]: 'KOI8-U',

    [EncodingID.Windows874]: 'Windows-874',
    [EncodingID.Windows1250]: 'Windows-1250',
    [EncodingID.Windows1251]: 'Windows-1251',
    [EncodingID.Windows1252]: 'Windows-1252',
    [EncodingID.Windows1253]: 'Windows-1253',
    [EncodingID.Windows1254]: 'Windows-1254',
    [EncodingID.Windows1255]: 'Windows-1255',
    [EncodingID.Windows1256]: 'Windows-1256',
    [EncodingID.Windows1257]: 'Windows-1257',
    [EncodingID.Windows1258]: 'Windows-1258',

    [EncodingID.MacRoman]: 'MacRoman',
    [EncodingID.MacCyrillic]: 'MacCyrillic',

    [EncodingID.ShiftJIS]: 'ShiftJIS',
    [EncodingID.EUCJP]: 'EUC-JP',
    [EncodingID.ISO2022JP]: 'ISO-2022-JP',

    [EncodingID.GBK]: 'GBK',
    [EncodingID.HZGB2312]: 'HZ-GB2312',
    [EncodingID.GB18030]: 'GB18030',

    [EncodingID.Big5]: 'Big5',

    [EncodingID.EUCKR]: 'EUC-KR'
});

/**
 * Dictionary equivalent to the Go map[string]EncodingID (nameToEncoding).
 *
 * Keys are lower-case, trimmed encoding names, including common aliases:
 *   "utf-8", "utf8", "shift-jis", "shiftjis", "gb18030", ...
 *
 * Values are EncodingID numeric codes.
 */
export const EncodingNameToId = Object.freeze({
    'utf-8': EncodingID.UTF8,
    'utf8': EncodingID.UTF8,
    'utf-16le': EncodingID.UTF16LE,
    'utf-16be': EncodingID.UTF16BE,
    'utf-16le-bom': EncodingID.UTF16LEBOM,
    'utf-16be-bom': EncodingID.UTF16BEBOM,

    'iso-8859-1': EncodingID.ISO8859_1,
    'iso-8859-2': EncodingID.ISO8859_2,
    'iso-8859-3': EncodingID.ISO8859_3,
    'iso-8859-4': EncodingID.ISO8859_4,
    'iso-8859-5': EncodingID.ISO8859_5,
    'iso-8859-6': EncodingID.ISO8859_6,
    'iso-8859-7': EncodingID.ISO8859_7,
    'iso-8859-8': EncodingID.ISO8859_8,
    'iso-8859-9': EncodingID.ISO8859_9,
    'iso-8859-10': EncodingID.ISO8859_10,
    'iso-8859-13': EncodingID.ISO8859_13,
    'iso-8859-14': EncodingID.ISO8859_14,
    'iso-8859-15': EncodingID.ISO8859_15,
    'iso-8859-16': EncodingID.ISO8859_16,

    'koi8-r': EncodingID.KOI8R,
    'koi8-u': EncodingID.KOI8U,

    'windows-874': EncodingID.Windows874,
    'windows-1250': EncodingID.Windows1250,
    'windows-1251': EncodingID.Windows1251,
    'windows-1252': EncodingID.Windows1252,
    'windows-1253': EncodingID.Windows1253,
    'windows-1254': EncodingID.Windows1254,
    'windows-1255': EncodingID.Windows1255,
    'windows-1256': EncodingID.Windows1256,
    'windows-1257': EncodingID.Windows1257,
    'windows-1258': EncodingID.Windows1258,

    'macroman': EncodingID.MacRoman,
    'maccyrillic': EncodingID.MacCyrillic,

    'shiftjis': EncodingID.ShiftJIS,
    'shift-jis': EncodingID.ShiftJIS,
    'euc-jp': EncodingID.EUCJP,
    'iso-2022-jp': EncodingID.ISO2022JP,

    'gbk': EncodingID.GBK,
    'hz-gb2312': EncodingID.HZGB2312,
    'gb18030': EncodingID.GB18030,

    'big5': EncodingID.Big5,

    'euc-kr': EncodingID.EUCKR
});

/**
 * Returns the canonical encoding name for an EncodingID.
 *
 * If the id is not known, returns "Unknown".
 *
 * @param {number} id
 * @returns {string}
 */
export function encodingName(id) {
    const name = EncodingIdToCanonicalName[id];
    return name || 'Unknown';
}

/**
 * Parses a name into an EncodingID, mirroring ParseEncoding(name string)
 * from Go. The lookup is case-insensitive and ignores leading/trailing
 * whitespace.
 *
 * Throws an Error when the encoding is unknown.
 *
 * @param {string} name
 * @returns {number} EncodingID
 */
export function parseEncoding(name) {
    const key = String(name).trim().toLowerCase();
    const id = EncodingNameToId[key];
    if (typeof id === 'undefined') {
        throw new Error(`unknown encoding: ${name}`);
    }
    return id;
}