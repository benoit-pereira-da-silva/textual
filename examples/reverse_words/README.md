# Reverse words

This example demonstrates how to build a small streaming pipeline with
[`textual`](https://github.com/benoit-pereira-da-silva/textual) that:

- reads an excerpt of **_Les Fleurs du mal_** by Charles Baudelaire,
- reverses every word while keeping punctuation and whitespaces in place,
- preserves capitalization patterns,
- streams the transformed text to standard output with a small random delay
  between each batch.

## How it works

The program wires together three building blocks from the `textual` package:

1. **`ProcessorFunc`** – implements the *reverse words* transformation.
   Each `Result` corresponds to one line of text.  
   For every line:
    - contiguous sequences of letters/digits are considered words,
    - the characters of each word are reversed,
    - the original casing pattern is reapplied position by position  
      (so a leading capital stays leading after the reversal),
    - punctuation and whitespace characters never move.

2. **`Chain`** – when the `--twice` flag is used, two reverse processors are
   composed in a `Chain`, so the text is reversed twice in a row.

3. **`IOReaderProcessor`** – streams the contents of
   `les_fleurs_du_mal.txt` line by line into the processor pipeline.

On the way out, the program waits for a random delay between **10ms** and
**100ms** after each processed line to emulate a live / streaming workload,
then renders the `Result` and prints it on `stdout`.

## Running the example

You can run the example with:

```shell
go run main.go
```
This reads the embedded [les_fleurs_du_mal.txt](files/les_fleurs_du_mal.txt) and prints a progressively transformed
version to the terminal.



To apply the reverse processor twice (using a Chain of two identical stages):
```shell
go run main.go --twice
```

You can also point the example at another UTF‑8 text file:
```shell
go run main.go --input=/path/to/your_text.txt --max-delay 100
```