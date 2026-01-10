# Unreleased
+ Introduced `Try, Catch, Finally` for procedural error handling.
+ Introduced `If,Then, Else, ElseIf` for procedural process branching.
+ Glue helpers function to compose a Transcoder and a Processor.
+ Introduced `textual.Transcoder` to transcode one Type to another. 
  + `textual.IOReaderTranscoder` connects an io.Reader to a Transcoder by scanning.
+ Added `textual.JSONCarrier` carrier
+ Added `ScanJSON` `bufio` split function.
+ Added error support to carriers (`WithError(err error)` / `GetError() error`) so processors can report per-item failures without breaking the stream.
+ Updated documentation and comments to reflect the Carrier / Parcel terminology.
+ Updated client helpers:
  + `helpers/js` now targets `Parcel` and includes a minimal `UTF8String` helper mirroring Go’s `textual.String`.
  + `helpers/swift` now targets `Parcel` and includes a minimal `UTF8String` helper mirroring Go’s `textual.String`.
+ Using `textual.String` in the [reverse_words example](examples/reverse_words/README.md).
+ Added `textual.String`, a minimal generic implementation of `textual.Carrier`.
+ Adapted `textual.Parcel` to implement `textual.Carrier`.
+ Exposed a generic interface `textual.Carrier`, and refactored the processing stack.
+ `textual.UTF8String` is now a symbolic string alias.
+ [Textual.swift](helpers/swift/README.md) a lightweight Swift utility to work with `textual` objects in native  clients.
+ [textual.js](helpers/js/README.md) a lightweight ES6 utility to work with `textual`  objects in the browser.

# 2025-12-11 v1.0.0

+ Original release of the textual Go toolkit.
+ [reverse words sample](examples/reverse_words/README.md)
