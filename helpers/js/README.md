# textual.js

Lightweight ES6 utilities to work with `textual` **carriers** in the browser.

This module is the client-side JavaScript counterpart of the Go `textual` package.
It focuses on **in-memory** manipulation of streamed values:

- `Parcel.rawTexts()` – compute the non-transformed parts of a text.
- `Parcel.render()` / `Parcel.utf8String()` – merge transformed fragments and raw text back into a single string.
- `StringCarrier` – a minimal carrier for plain UTF‑8 text + index + error (mirrors Go’s `textual.StringCarrier`).
- `JsonCarrier` – a minimal carrier for raw JSON values + index + error (mirrors Go’s `textual.JsonCarrier`).
- `scanJSON(...)` / `scanJSONBytes(...)` – split a stream into top-level JSON values (mirrors Go’s `ScanJSON` split func).
- Encoding helpers that mirror the Go `EncodingID` enum and `nameToEncoding` map.

No transport is included: you plug this into whatever you use already
(SSE, WebSocket, fetch, etc.).

## Important note about JSON carriers

The Go package provides both:

- `JsonCarrier` (dynamic, raw JSON bytes)
- `JsonGenericCarrier[T]` (typed JSON carrier)

This JS helper intentionally **does not** implement a generic/typed JSON carrier.
For JavaScript, use **`JsonCarrier`** and parse/cast at the edges with `JSON.parse(...)`
(or `JsonCarrier.parseValue()`).

## Features

- ✅ ES6 module, browser-first (no Node.js dependencies at runtime).
- ✅ One-file utility, easy to drop into any frontend app.
- ✅ API mirrors Go types: `Parcel`, `Fragment`, `RawText`, `StringCarrier`, `JsonCarrier`.
- ✅ `rawTexts()` and `render()` aligned with the Go implementation.
- ✅ `scanJSON(...)` / `scanJSONBytes(...)` aligned with the Go implementation.
- ✅ `EncodingID`, `EncodingNameToId`, `EncodingIdToCanonicalName`.
- ✅ `parseEncoding(name)` and `encodingName(id)` helpers.

## Installation

Copy the file (for example `textual.js`) into your project and import it
as an ES module.

### Direct usage in the browser

```html
<script type="module">
  import { Parcel, parcelFrom } from './textual.js';

  const p = parcelFrom('Hello, café');
  console.log(p.render());
</script>
```

### With a bundler

If you use Vite, Webpack, Rollup, etc., place `textual.js` somewhere in
your source tree and import it:

```js
// src/app.js
import { Parcel, parcelFrom, StringCarrier, stringFrom } from './textual.js';

const p = parcelFrom('Hello, café');
console.log(p.render());

const s = stringFrom('plain text');
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

### StringCarrier model (minimal carrier)

When you don’t need fragments/variants, you can use `StringCarrier`, which mirrors
Go’s minimal carrier `textual.StringCarrier`:

```js
class StringCarrier {
  value; // string
  index; // number
  error; // string | null
}
```

This helper intentionally stays simple: it’s meant to carry plain UTF‑8 text
plus an optional ordering hint and a portable error string.

> Compatibility: older versions exposed this class as `UTF8String`.
> `UTF8String` is still exported as a deprecated alias of `StringCarrier`.

### JsonCarrier model (minimal JSON carrier)

When you want to stream JSON values through your frontend pipeline:

```js
class JsonCarrier {
  value; // string (raw JSON text, one top-level value)
  index; // number
  error; // string | null
}
```

- `value` is stored as raw JSON text (e.g. `{"a":1}`, `[1,2]`, `"hello"`, `42`, `true`, `null`).
- `JsonCarrier.fromJSON(payload)` accepts the **Go wire shape** where `value` is the JSON value itself
  (object/array/string/number/bool/null), and converts it to raw JSON text internally.
- Use `j.utf8String()` to get the raw JSON text back.
- Use `j.parseValue()` (or `JSON.parse(j.utf8String())`) to parse the value when needed.

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
import { parcelFrom } from './textual.js';

const p = parcelFrom('Plain text only');
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
import { Parcel, JsonCarrier } from './textual.js';

const source = new EventSource('/textual/stream');

source.addEventListener('parcel', (event) => {
  try {
    const payload = JSON.parse(event.data);
    const p = Parcel.fromJSON(payload);

    const rawSegments = p.rawTexts();
    const text = p.render();

    appendToUI(text, rawSegments, p.error);
  } catch (err) {
    console.error('Failed to handle textual parcel', err);
  }
});

source.addEventListener('json', (event) => {
  try {
    const payload = JSON.parse(event.data);
    const j = JsonCarrier.fromJSON(payload);

    // Raw JSON (UTF-8 text)
    const raw = j.utf8String();

    // Parsed JS value (object/array/string/number/bool/null)
    const parsed = j.parseValue();
    if (parsed.ok) {
      handleJSON(parsed.value);
    } else {
      console.error('Invalid JSON value in JsonCarrier', parsed.error);
    }
  } catch (err) {
    console.error('Failed to handle textual json carrier', err);
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
