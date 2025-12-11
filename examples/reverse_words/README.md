# Reverse words

This example demonstrates how to build a small streaming pipeline with
[`textual`](https://github.com/benoit-pereira-da-silva/textual) that:

- reads an excerpt of **_Les Fleurs du mal_** by Charles Baudelaire,
- reverses every word while keeping punctuation and whitespaces in place,
- preserves capitalization patterns,
- streams the transformed text to standard output with a small random delay
  between each batch.

You can run it line‑by‑line (default) or expression‑by‑expression (word +
punctuation + surrounding spaces / line breaks).

---

## How it works

The program wires together several building blocks from the `textual` package:

1. **`ProcessorFunc`** – implements the *reverse words* transformation.

   Each `Result` corresponds to one *token* of text (by default a line).

   For every token:

   - contiguous sequences of letters/digits are considered words,
   - the characters of each word are reversed,
   - the original casing pattern is reapplied position by position  
     (so a leading capital stays leading after the reversal),
   - punctuation and whitespace characters never move.

2. **`Chain`** – when the `--twice` flag is used, two reverse processors are
   composed in a `Chain`, so the text is reversed twice in a row.

3. **`IOReaderProcessor`** – streams text from an `io.Reader` into the pipeline.

   - By default, it uses `bufio.ScanLines`, so each `Result` is one line from
     `les_fleurs_du_mal.txt`.
   - When the `--word-by-word` flag is set, the example switches to
     `textual.ScanExpression`, a custom split function that yields *expressions*
     of the form:

     ```text
     [optional leading whitespace]
     [word + punctuation]
     [optional trailing whitespace / newlines]
     ```

     Concatenating all expressions in order reconstructs the original text,
     which means the example can simply `fmt.Print` each piece as it arrives
     without guessing where to add spaces or `\n`.

4. **Random delay** – after each processed token the program waits for a
   random delay between `min-delay` and `max-delay` milliseconds to emulate a
   live / streaming workload.

---

## Running the example

From the `examples/reverse_words` directory:

### Line-by-line (default)

```shell
go run main.go
```

This reads the embedded [les_fleurs_du_mal.txt](files/les_fleurs_du_mal.txt)
and progressively prints a transformed version to the terminal, one line at a
time.

### Expression-by-expression (word + punctuation + spaces/newlines)

```shell
go run main.go --word-by-word
```

In this mode the input is tokenized with `textual.ScanExpression`. Each piece
contains a word, the punctuation around it, and the spaces or line breaks that
follow it. The output still reconstructs the original layout, but you see it
arrive in much smaller chunks.

### Reverse twice

```shell
go run main.go --twice
```

This chains the reverse processor twice using `textual.NewChain`, so the text
is reversed twice in a row and ends up identical to the original (up to the
casing rules described above).

### Use another input file

```shell
go run main.go --input=/path/to/your_text.txt --max-delay=250
```

Flags you can use:

- `--input`       — path to a UTF‑8 text file (defaults to the embedded excerpt),
- `--twice`       — apply the reverse processor twice,
- `--word-by-word` — stream one expression at a time using `textual.ScanExpression`,
- `--min-delay`   — minimum delay between processed tokens in milliseconds,
- `--max-delay`   — maximum delay between processed tokens in milliseconds.
