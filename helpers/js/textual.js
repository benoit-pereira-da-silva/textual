// textual.js
// Lightweight ES6 utilities to work with "textual" Parcel objects in a browser.
//
// This file mirrors the behavior of the Go "textual" package for:
//   - Parcel.RawTexts()
//   - Parcel.UTF8String() (exposed here as render()/utf8String())
// and exposes an EncodingID map plus helpers similar to encoding.go.
//
// It also provides a minimal UTF8String helper mirroring Go's textual.String,
// useful when you only need plain UTF‑8 text + index + error in client code.
//
// It is transport-agnostic: it assumes you already receive Parcel-like JSON
// objects (for example, from SSE, WebSocket, or fetch) and helps you manipulate
// them in the browser.
//
// Usage (ES modules):
//   import { Parcel, UTF8String, input, utf8String, EncodingID, parseEncoding, encodingName } from './textual.js';
//
//   const p = input('Hello, café');
//   const rawParts = p.rawTexts();
//   const rendered = p.render();
//
//   const s = utf8String('plain text');
//   console.log(s.utf8String());
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
 * @typedef {string} UTF8Text
 * Documentation-only alias: all strings are assumed to be valid UTF-8.
 */

/**
 * normalizeError turns any error-like value into a portable string.
 *
 * The Go side serializes `error` as a string, so this helper keeps the JS side
 * consistent by storing errors as strings (or null).
 *
 * @param {*} err
 * @returns {string|null}
 */
function normalizeError(err) {
    if (err === null || typeof err === 'undefined') {
        return null;
    }
    if (err instanceof Error) {
        return err.message || String(err);
    }
    const s = String(err);
    return s.length > 0 ? s : null;
}

/**
 * joinErrorStrings merges two error strings into a single string.
 *
 * This mirrors the "error join" intent from Go's errors.Join, but keeps the
 * representation simple and portable for JSON clients.
 *
 * @param {string|null} a
 * @param {string|null} b
 * @returns {string|null}
 */
function joinErrorStrings(a, b) {
    const aa = normalizeError(a);
    const bb = normalizeError(b);
    if (!aa) return bb;
    if (!bb) return aa;
    if (aa === bb) return aa;
    return `${aa}; ${bb}`;
}

/**
 * isTrailingPunctOrSpace reports whether a single code point should be considered
 * removable at the end of a rendered line for UX purposes.
 *
 * This is intentionally conservative: it trims whitespace and common punctuation
 * or closing quote/bracket characters.
 *
 * @param {string} ch
 * @returns {boolean}
 */
function isTrailingPunctOrSpace(ch) {
    if (!ch) return false;
    if (/\s/.test(ch)) return true;
    const punct = ".,;:!?…»«\"'’)]}";
    return punct.indexOf(ch) !== -1;
}

/**
 * Trim trailing punctuation/spaces from a rendered line, in code-point space.
 *
 * @param {string} line
 * @returns {string}
 */
function trimLineEndPunctOrSpace(line) {
    const runes = Array.from(String(line ?? ""));
    let end = runes.length;
    while (end > 0 && isTrailingPunctOrSpace(runes[end - 1])) {
        end--;
    }
    return runes.slice(0, end).join("");
}

/**
 * Fragment represents a transformed portion of the original text.
 *
 * Positions (pos) and lengths (len) are expressed in Unicode code points,
 * not bytes. This matches the Go implementation which operates in rune space.
 */
export class Fragment {
    /**
     * @param {Object} opts
     * @param {UTF8Text} [opts.transformed] - Transformed text (IPA, SAMPA, etc.).
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
        /** @type {UTF8Text} */
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
 * It is computed from a Parcel and covers every range that is not overlapped
 * by any Fragment (after merging overlapping fragments).
 */
export class RawText {
    /**
     * @param {Object} opts
     * @param {UTF8Text} [opts.text] - Raw text content.
     * @param {number} [opts.pos] - First code-point position in the original text.
     * @param {number} [opts.len] - Length in code points.
     */
    constructor({ text = '', pos = 0, len = 0 } = {}) {
        /** @type {UTF8Text} */
        this.text = String(text);
        /** @type {number} */
        this.pos = Number.isFinite(pos) ? Math.trunc(pos) : 0;
        /** @type {number} */
        this.len = Number.isFinite(len) ? Math.trunc(len) : 0;
    }
}

/**
 * UTF8String is the minimal "carrier" helper, mirroring Go's textual.String.
 *
 * Use it when you only need:
 *   - a UTF‑8 string (value)
 *   - an optional ordering hint (index)
 *   - an optional, portable error string (error)
 *
 * This helper is intentionally small and does NOT implement Parcel-like
 * fragment logic.
 */
export class UTF8String {
    /**
     * @param {Object} opts
     * @param {UTF8Text} [opts.value] - UTF‑8 text.
     * @param {number} [opts.index] - Optional index in a stream.
     * @param {string|null} [opts.error] - Optional error string.
     */
    constructor({ value = '', index = 0, error = null } = {}) {
        /** @type {UTF8Text} */
        this.value = String(value);
        /** @type {number} */
        this.index = Number.isFinite(index) ? Math.trunc(index) : 0;
        /** @type {string|null} */
        this.error = normalizeError(error);
    }

    /**
     * Builds a UTF8String from JSON.
     *
     * Accepts both lower-case and Go-style exported keys:
     *   - value / Value
     *   - index / Index
     *   - error / Error
     *
     * @param {Object|UTF8String} json
     * @returns {UTF8String}
     */
    static fromJSON(json) {
        if (json instanceof UTF8String) {
            return json;
        }
        if (!json || typeof json !== 'object') {
            return new UTF8String();
        }
        return new UTF8String({
            value: json.value ?? json.Value ?? '',
            index: json.index ?? json.Index ?? 0,
            error: json.error ?? json.Error ?? null
        });
    }

    /**
     * Serializes the value to a JSON-friendly object.
     *
     * @returns {{value: string, index: number, error: (string|null)}}
     */
    toJSON() {
        return {
            value: this.value,
            index: this.index,
            error: this.error
        };
    }

    /**
     * Returns the UTF‑8 text (Go: UTF8String()).
     *
     * @returns {UTF8Text}
     */
    utf8String() {
        return this.value;
    }

    /**
     * Creates a new UTF8String from a UTF‑8 token (Go: FromUTF8String()).
     *
     * @param {UTF8Text} text
     * @returns {UTF8String}
     */
    fromUTF8String(text) {
        return new UTF8String({ value: String(text), index: 0, error: null });
    }

    /**
     * Returns a copy of the value with its index set (Go: WithIndex()).
     *
     * @param {number} index
     * @returns {UTF8String}
     */
    withIndex(index) {
        return new UTF8String({
            value: this.value,
            index: Number.isFinite(index) ? Math.trunc(index) : 0,
            error: this.error
        });
    }

    /**
     * Returns the stored index (Go: GetIndex()).
     *
     * @returns {number}
     */
    getIndex() {
        return this.index;
    }

    /**
     * Returns a copy of the value with its error merged (Go: WithError()).
     *
     * @param {*} err
     * @returns {UTF8String}
     */
    withError(err) {
        const merged = joinErrorStrings(this.error, err);
        return new UTF8String({
            value: this.value,
            index: this.index,
            error: merged
        });
    }

    /**
     * Returns the stored error (Go: GetError()).
     *
     * @returns {string|null}
     */
    getError() {
        return this.error;
    }

    /**
     * Aggregates multiple UTF8String values into one.
     *
     * Behaviour mirrors Go's textual.String.Aggregate:
     *   - Items are copied and stably sorted by index.
     *   - When indices are equal, value is used as a tie-breaker.
     *   - The output index is reset to 0.
     *   - Errors are merged into a single portable string.
     *
     * @param {UTF8String[]} items
     * @returns {UTF8String}
     */
    aggregate(items) {
        const list = (items || []).map((it) => UTF8String.fromJSON(it));

        list.sort((a, b) => {
            if (a.index !== b.index) {
                return a.index - b.index;
            }
            // Tie-breaker for deterministic ordering.
            return a.value < b.value ? -1 : a.value > b.value ? 1 : 0;
        });

        let out = '';
        let err = null;
        for (const it of list) {
            out += it.value;
            err = joinErrorStrings(err, it.error);
        }
        return new UTF8String({ value: out, index: 0, error: err });
    }
}

/**
 * Parcel is the rich value in the textual pipeline.
 *
 * It mirrors the Go struct (textual.Parcel) and supports:
 *   - rawTexts(): compute the untouched spans of the original text
 *   - render(): merge fragments and raw text back into a single string
 *
 * In JavaScript:
 *   - `text` is a UTF‑8 string.
 *   - `fragments` is an array of Fragment instances.
 *   - `error` is kept as an opaque value (usually a string) for portability.
 */
export class Parcel {
    /**
     * @param {Object} opts
     * @param {number} [opts.index] - Optional index in a stream.
     * @param {UTF8Text} [opts.text] - Original UTF‑8 text.
     * @param {Array<Fragment|Object>} [opts.fragments] - Fragments describing transformed regions.
     * @param {*} [opts.error] - Optional error; not interpreted by this module.
     */
    constructor({ index = -1, text = '', fragments = [], error = null } = {}) {
        /** @type {number} */
        this.index = Number.isFinite(index) ? Math.trunc(index) : -1;
        /** @type {UTF8Text} */
        this.text = String(text);
        /** @type {Fragment[]} */
        this.fragments = (fragments || []).map((f) =>
            f instanceof Fragment ? f : new Fragment(f)
        );
        /** @type {*} */
        this.error = error;
    }

    /**
     * Constructs a Parcel from a plain JSON object that follows the same
     * shape as the Go struct when serialized to JSON.
     *
     * @param {Object|Parcel} json
     * @returns {Parcel}
     */
    static fromJSON(json) {
        if (json instanceof Parcel) {
            return json;
        }
        if (!json || typeof json !== 'object') {
            return new Parcel();
        }
        return new Parcel({
            index: json.index ?? json.Index ?? -1,
            text: json.text ?? json.Text ?? '',
            fragments: json.fragments ?? json.Fragments ?? [],
            error: json.error ?? json.Error ?? null
        });
    }

    /**
     * Aggregate multiple parcels into one, sorting by index (stable).
     *
     * This is designed for streaming use-cases where chunks may arrive out of order.
     *
     * @param {Array<Parcel|Object>} parcels
     * @returns {Parcel}
     */
    static aggregateByIndex(parcels) {
        return new Parcel().aggregateByIndex(parcels);
    }

    /**
     * Serializes the Parcel to a plain JSON-friendly object.
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
     * Carrier-like convenience: returns a copy with the index set.
     *
     * @param {number} index
     * @returns {Parcel}
     */
    withIndex(index) {
        return new Parcel({
            index: Number.isFinite(index) ? Math.trunc(index) : -1,
            text: this.text,
            fragments: this.fragments,
            error: this.error
        });
    }

    /**
     * Carrier-like convenience: returns the index.
     *
     * @returns {number}
     */
    getIndex() {
        return this.index;
    }

    /**
     * Carrier-like convenience: returns a copy with an error merged.
     *
     * The error is stored as an opaque value, but when both existing and new
     * errors are string-like they are concatenated.
     *
     * @param {*} err
     * @returns {Parcel}
     */
    withError(err) {
        const a = normalizeError(this.error);
        const b = normalizeError(err);
        const merged = joinErrorStrings(a, b);
        return new Parcel({
            index: this.index,
            text: this.text,
            fragments: this.fragments,
            error: merged ?? this.error ?? err
        });
    }

    /**
     * Carrier-like convenience: returns the stored error.
     *
     * @returns {*}
     */
    getError() {
        return this.error;
    }

    /**
     * Carrier-like convenience: builds a new Parcel from a UTF‑8 token.
     *
     * @param {UTF8Text} text
     * @returns {Parcel}
     */
    fromUTF8String(text) {
        return new Parcel({ index: -1, text: String(text), fragments: [], error: null });
    }

    /**
     * Returns a copy of this Parcel with a new fragments array.
     *
     * @param {Array<Fragment|Object>} fragments
     * @returns {Parcel}
     */
    withFragments(fragments) {
        return new Parcel({
            index: this.index,
            text: this.text,
            fragments: fragments || [],
            error: this.error
        });
    }

    /**
     * Returns a copy of the Parcel where, for each exact {pos,len} range, only the
     * "best" fragment is kept (highest confidence, tie-breaker: lowest variant).
     *
     * Useful when a backend emits multiple candidate variants for a single segment.
     *
     * @returns {Parcel}
     */
    bestConfidenceVariantByRange() {
        /** @type {Map<string, Fragment>} */
        const bestByRange = new Map();

        const pickBetter = (a, b) => {
            // Prefer higher confidence
            if (a.confidence !== b.confidence) {
                return a.confidence > b.confidence ? a : b;
            }
            // Prefer lower variant index (stable UI expectations)
            if (a.variant !== b.variant) {
                return a.variant < b.variant ? a : b;
            }
            // Deterministic tie-breaker
            const at = String(a.transformed);
            const bt = String(b.transformed);
            return at <= bt ? a : b;
        };

        for (const f of (this.fragments || [])) {
            if (!f || !Number.isFinite(f.pos) || !Number.isFinite(f.len)) continue;
            const pos = Math.trunc(f.pos);
            const len = Math.trunc(f.len);
            if (len <= 0) continue;

            const key = `${pos}:${len}`;
            const cur = bestByRange.get(key);
            if (!cur) {
                bestByRange.set(key, f);
                continue;
            }
            bestByRange.set(key, pickBetter(cur, f));
        }

        // Emit a deterministic order by position.
        const chosen = Array.from(bestByRange.values()).map((f) => new Fragment({
            transformed: f.transformed,
            pos: f.pos,
            len: f.len,
            confidence: f.confidence,
            variant: f.variant
        }));

        chosen.sort((a, b) => {
            if (a.pos !== b.pos) return a.pos - b.pos;
            if (a.len !== b.len) return a.len - b.len;
            return a.variant - b.variant;
        });

        return new Parcel({
            index: this.index,
            text: this.text,
            fragments: chosen,
            error: this.error
        });
    }

    /**
     * Render this Parcel with optional validation and line trimming.
     *
     * Options:
     *  - variantPolicy: "default" | "bestConfidence"
     *      - "default" uses the same behaviour as render().
     *      - "bestConfidence" selects one fragment per {pos,len} range using confidence/variant.
     *  - trimLineEnd: when true, removes trailing punctuation/spaces per output line.
     *  - normalizeWhitespaceOnlyLines: when true, lines that are whitespace-only in the original
     *    text are rendered as empty lines.
     *
     * @param {{variantPolicy?: ("default"|"bestConfidence"), trimLineEnd?: boolean, normalizeWhitespaceOnlyLines?: boolean}} [options]
     * @returns {UTF8Text}
     */
    renderWith(options = {}) {
        const opts = (options && typeof options === "object") ? options : {};
        const variantPolicy = String(opts.variantPolicy ?? "default");
        const trimLineEnd = !!opts.trimLineEnd;
        const normalizeWhitespaceOnlyLines = !!opts.normalizeWhitespaceOnlyLines;

        const base = (variantPolicy === "bestConfidence")
            ? this.bestConfidenceVariantByRange()
            : this;

        const rendered = base.render();

        if (!trimLineEnd && !normalizeWhitespaceOnlyLines) {
            return rendered;
        }

        const originalLines = String(this.text ?? "").split("\n");
        const outLines = String(rendered ?? "").split("\n");

        const max = Math.max(originalLines.length, outLines.length);
        /** @type {string[]} */
        const finalLines = [];

        for (let i = 0; i < max; i++) {
            const origLine = (i < originalLines.length) ? originalLines[i] : "";
            const outLine = (i < outLines.length) ? outLines[i] : "";

            if (normalizeWhitespaceOnlyLines && origLine.trim() === "") {
                finalLines.push("");
                continue;
            }

            if (trimLineEnd) {
                finalLines.push(trimLineEndPunctOrSpace(outLine));
            } else {
                finalLines.push(outLine);
            }
        }

        return finalLines.join("\n");
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
     * Rules (matching the Go UTF8String/Render behaviour):
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
     * @returns {UTF8Text}
     */
    render() {
        /**
         * @typedef {{pos: number, text: UTF8Text}} Segment
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

    /**
     * Alias for render(), matching the Carrier terminology (Go: UTF8String()).
     *
     * @returns {UTF8Text}
     */
    utf8String() {
        return this.render();
    }

    /**
     * Aggregates multiple parcels into a single parcel by concatenating their
     * `text` fields and offsetting fragment positions.
     *
     * This is a convenience that mirrors the intent of Go Carrier.Aggregate for
     * the Parcel shape:
     *   - texts are concatenated in the provided order
     *   - fragment positions are shifted by the code-point length of the
     *     preceding texts
     *   - errors are merged into a single portable string when possible
     *
     * @param {Parcel[]} parcels
     * @returns {Parcel}
     */
    aggregate(parcels) {
        const list = (parcels || []).map((p) => Parcel.fromJSON(p));

        let text = '';
        let offset = 0;
        /** @type {Fragment[]} */
        const fragments = [];
        let err = null;

        for (const p of list) {
            const pText = String(p.text);
            const pLen = Array.from(pText).length;

            // Merge errors.
            err = joinErrorStrings(err, p.error);

            // Copy fragments with shifted positions.
            if (Array.isArray(p.fragments)) {
                for (const f of p.fragments) {
                    if (!f) continue;
                    fragments.push(
                        new Fragment({
                            transformed: f.transformed,
                            pos: (Number.isFinite(f.pos) ? Math.trunc(f.pos) : 0) + offset,
                            len: Number.isFinite(f.len) ? Math.trunc(f.len) : 0,
                            confidence: Number.isFinite(f.confidence) ? f.confidence : 0,
                            variant: Number.isFinite(f.variant) ? Math.trunc(f.variant) : 0
                        })
                    );
                }
            }

            text += pText;
            offset += pLen;
        }

        return new Parcel({
            index: -1,
            text,
            fragments,
            error: err
        });
    }

    /**
     * Aggregates multiple parcels into a single parcel by sorting the inputs by their index.
     *
     * This is the streaming-friendly variant of aggregate():
     *  - Parcels are parsed and then stably sorted by index.
     *  - When indices are equal, original arrival order is preserved.
     *
     * @param {Array<Parcel|Object>} parcels
     * @returns {Parcel}
     */
    aggregateByIndex(parcels) {
        const decorated = (parcels || []).map((p, order) => ({
            p: Parcel.fromJSON(p),
            order: Number.isFinite(order) ? order : 0
        }));

        decorated.sort((a, b) => {
            const ia = Number.isFinite(a.p.index) ? a.p.index : -1;
            const ib = Number.isFinite(b.p.index) ? b.p.index : -1;

            if (ia !== ib) return ia - ib;
            return a.order - b.order;
        });

        const sorted = decorated.map((x) => x.p);
        return this.aggregate(sorted);
    }
}

/**
 * Convenience factory mirroring the "create from UTF‑8 text" pattern.
 *
 * Creates a base Parcel with:
 *   - index = -1
 *   - text = given argument
 *   - fragments = []
 *   - error = null
 *
 * @param {UTF8Text} text
 * @returns {Parcel}
 */
export function input(text) {
    return new Parcel({
        index: -1,
        text: String(text),
        fragments: [],
        error: null
    });
}

/**
 * Convenience factory for the minimal UTF8String carrier helper.
 *
 * Creates a base UTF8String with:
 *   - value = given argument
 *   - index = 0
 *   - error = null
 *
 * @param {UTF8Text} text
 * @returns {UTF8String}
 */
export function utf8String(text) {
    return new UTF8String({
        value: String(text),
        index: 0,
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