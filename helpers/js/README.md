# textual.js

Lightweight ES6 utilities to work with `textual` **`Parcel`** objects in the browser.

This module is the client-side JavaScript counterpart of the Go `textual`
package. It focuses on **in-memory** manipulation of streamed values:

- `Parcel.rawTexts()` – compute the non-transformed parts of a text.
- `Parcel.render()` / `Parcel.utf8String()` – merge transformed fragments and raw text back into a single string.
- A minimal `UTF8String` helper (mirrors Go’s `textual.String`) when you only need plain text + index + error.
- Encoding helpers that mirror the Go `EncodingID` enum and `nameToEncoding` map.

No transport is included: you plug this into whatever you use already
(SSE, WebSocket, fetch, etc.).

## Features

- ✅ ES6 module, browser-first (no Node.js dependencies).
- ✅ One-file utility, easy to drop into any frontend app.
- ✅ API mirrors Go types: `Parcel`, `Fragment`, `RawText`, `UTF8String`.
- ✅ `rawTexts()` and `render()` aligned with the Go implementation.
- ✅ `EncodingID`, `EncodingNameToId`, `EncodingIdToCanonicalName`.
- ✅ `parseEncoding(name)` and `encodingName(id)` helpers.

## Installation

Copy the file (for example `textual.js`) into your project and import it
as an ES module.

### Direct usage in the browser

```html
<script type="module">
  import { Parcel, input } from './textual.js';

  const p = input('Hello, café');
  console.log(p.render());
</script>
```

### With a bundler

If you use Vite, Webpack, Rollup, etc., place `textual.js` somewhere in
your source tree and import it:

```js
// src/app.js
import { Parcel, input, UTF8String, utf8String } from './textual.js';

const p = input('Hello, café');
console.log(p.render());

const s = utf8String('plain text');
console.log(s.utf8String());
```

## Data model

The JS module mirrors the Go structs.

### Parcel model (rich carrier)

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

// Parcel (Go: textual.Parcel)
class Parcel {
  index;     // number (optional index in a stream)
  text;      // original text (UTF-8 string)
  fragments; // Fragment[]
  error;     // arbitrary value (usually a string or null)
}
```

`Parcel` is useful when your backend streams “partial transformations”
(fragments, variants, confidence, raw segments…).

### UTF8String model (minimal carrier)

When you don’t need fragments/variants, you can use `UTF8String`, which mirrors
Go’s minimal carrier `textual.String`:

```js
class UTF8String {
  value; // string
  index; // number
  error; // string | null
}
```

This helper intentionally stays simple: it’s meant to carry plain UTF‑8 text
plus an optional ordering hint and a portable error string.

## RawTexts / Render

You can feed `Parcel` either from your own code or from JSON coming from your Go backend.

```js
import { Parcel } from './textual.js';

// From JSON (e.g. SSE payload)
const json = {
  text: 'Hello, café',
  fragments: [
    { transformed: 'həˈloʊ', pos: 0, len: 5, confidence: 0.9, variant: 0 },
  ],
};

const p = Parcel.fromJSON(json);

// Raw segments of the original text that are NOT covered by a fragment.
const rawParts = p.rawTexts();  // -> RawText[]

// Final merged string: fragments + raw text, ordered by their positions.
const rendered = p.render();    // -> "həˈloʊ, café"
```

There is also a small convenience factory that mirrors the Go “create from text”
pattern:

```js
import { input } from './textual.js';

const p = input('Plain text only');
console.log(p.render());  // "Plain text only"
```

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
import { Parcel } from './textual.js';

const source = new EventSource('/textual/stream');

source.addEventListener('parcel', (event) => {
  try {
    const payload = JSON.parse(event.data);
    const p = Parcel.fromJSON(payload);

    // 1. Inspect raw segments (for highlighting, for example).
    const rawSegments = p.rawTexts();

    // 2. Render the final string.
    const text = p.render();

    // 3. Do whatever you want with it (append to the DOM, etc.).
    appendToUI(text, rawSegments, p.error);
  } catch (err) {
    console.error('Failed to handle textual parcel', err);
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
