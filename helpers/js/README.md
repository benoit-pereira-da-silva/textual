# textual.js

Lightweight ES6 utilities to work with `textual` `Result` objects in the browser.

This module is the client-side JavaScript counterpart of the Go `textual`
package. It focuses on **in-memory** manipulation of results:

- `Result.rawTexts()` – compute the non-transformed parts of a text.
- `Result.render()` – merge transformed fragments and raw text back into a single string.
- Encoding helpers that mirror the Go `EncodingID` enum and `nameToEncoding` map.

No transport is included: you plug this into whatever you use already
(SSE, WebSocket, fetch, etc.).

## Features

- ✅ ES6 module, browser-first (no Node.js dependencies).
- ✅ One-file utility, easy to drop into any frontend app.
- ✅ API mirrors Go types: `Result`, `Fragment`, `RawText`.
- ✅ `rawTexts()` and `render()` aligned with the Go implementation.
- ✅ `EncodingID`, `EncodingNameToId`, `EncodingIdToCanonicalName`.
- ✅ `parseEncoding(name)` and `encodingName(id)` helpers.

## Installation

Copy the file (for example `textual.js`) into your project and import it
as an ES module.

### Direct usage in the browser

```html
<script type="module">
  import { Result } from './textual.js';

  const res = new Result({ text: 'Hello, café' });
  console.log(res.render());
</script>
```

### With a bundler

If you use Vite, Webpack, Rollup, etc., place `textual.js` somewhere in
your source tree and import it:

```js
// src/app.js
import { Result, input } from './textual.js';

const res = input('Hello, café');
console.log(res.render());
```

## Data model

The JS module mirrors the Go structs:

```js
// Fragment
// Positions and lengths are expressed in "character" units (Unicode code points),
// not bytes.
class Fragment {
  transformed; // string
  pos;         // number (start index in original text)
  len;         // number (length in characters)
  confidence;  // number
  variant;     // number
}

// RawText
class RawText {
  text; // string
  pos;  // number
  len;  // number
}

// Result
class Result {
  index;     // number (optional index in a stream)
  text;      // original text (UTF-8 string)
  fragments; // Fragment[]
  error;     // arbitrary value (usually a string or null)
}
```

In practice you never need to instantiate these classes manually unless you
want to – the utilities are designed to play nicely with JSON coming from
your Go backend.

## RawTexts / Render

You can feed `Result` either from your own code or from JSON coming from your Go backend.

```js
import { Result } from './textual.js';

// From JSON (e.g. SSE payload)
const json = {
  text: 'Hello, café',
  fragments: [
    { transformed: 'həˈloʊ', pos: 0, len: 5, confidence: 0.9, variant: 0 },
  ],
};

const res = Result.fromJSON(json);

// Raw segments of the original text that are NOT covered by a fragment.
const rawParts = res.rawTexts();  // -> RawText[]

// Final merged string: fragments + raw text, ordered by their positions.
const rendered = res.render();    // -> "həˈloʊ, café"
```

There is also a small convenience factory that mirrors the Go `Input` function:

```js
import { input } from './textual.js';

const res = input('Plain text only');
console.log(res.render());  // "Plain text only"
```

### How RawTexts works (short version)

- All positions/lengths are interpreted in **code points** (characters), not
  UTF‑8 bytes.
- Fragments are sorted and merged into a set of non-overlapping ranges.
- Every gap between those ranges becomes a `RawText` segment.
- This lets you reconstruct the final output by mixing:
  - `Fragment.transformed`
  - `RawText.text`

## Encoding dictionary

You can query encodings by name or by ID without dealing with any transport
or actual transcoding:

```js
import { EncodingID, parseEncoding, encodingName } from './textual.js';

const id = parseEncoding('utf-8');        // -> EncodingID.UTF8
const humanLabel = encodingName(id);      // -> "UTF-8"
const another = EncodingID.Windows1252;   // numeric code (24)
```

Internally, the module exposes two dictionaries:

- `EncodingNameToId`: lower-case names (with aliases such as `"utf8"`,
  `"shift-jis"`, `"windows-1252"`) mapped to numeric IDs.
- `EncodingIdToCanonicalName`: numeric IDs mapped back to canonical names
  such as `"UTF-8"`, `"ISO-8859-1"`.

This mirrors the Go `EncodingID` enum and `nameToEncoding` map so you can
safely pass numeric IDs back and forth between Go, JS, and (if you use it)
the Swift helper.

## Working with streaming backends (example)

Here is how you might integrate this with Server-Sent Events (SSE). The
transport itself is **not** part of this module; this is just an example.

```js
import { Result } from './textual.js';

const source = new EventSource('/textual/stream');

source.addEventListener('result', (event) => {
  try {
    const payload = JSON.parse(event.data);
    const res = Result.fromJSON(payload);

    // 1. Inspect raw segments (for highlighting, for example).
    const rawSegments = res.rawTexts();

    // 2. Render the final string.
    const text = res.render();

    // 3. Do whatever you want with it (append to the DOM, etc.).
    appendToUI(text, rawSegments);
  } catch (err) {
    console.error('Failed to handle textual result', err);
  }
});
```

You can use the exact same pattern with WebSocket messages or `fetch()` +
`ReadableStream` if you already have streaming in place.

## Notes on indexing

- The Go side uses rune indices (`[]rune`), so positions and lengths are in
  Unicode code points.
- The JS helper uses `Array.from(string)` to emulate the same behaviour, so
  `pos`/`len` should be interpreted as **characters**, not bytes.
- If you compute fragment positions in JavaScript, always count in characters
  the same way the Go backend does, or simply let the backend provide `pos`
  and `len`.

## License

Same license as the Go package that defines the original `textual` types
(Apache 2.0 in the original source).
