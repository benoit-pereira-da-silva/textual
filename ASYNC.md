# Async and PanicStore (textual)

This document explains how to use `textual.Async` and `textual.WithPanicStore` to build **streaming** processors and transcoders that communicate through **channels** and are interruptible via **context cancellation**.
The design is intentionally “low ceremony” and close to Go’s idioms, but it is also **unforgiving** if the pipeline is not built with the required discipline.

---

## Mental model

Think “Unix pipes”, but typed:

- **A stage** is typically **one goroutine**.
- **A stream** is a **typed receive-only channel** (`<-chan T`).
- **End-of-stream** is signaled by **closing the input channel**.
- **Early stop / interruption** is signaled by **canceling a shared context**.
- **Backpressure** is inherent: if a downstream stage is slow, upstream stages will eventually block on sends.
- **Per-item errors** are **data** (carried by your `carrier.Carrier` implementation via `WithError/GetError`).
- **Panics** are treated as **fatal faults** and are captured **out-of-band** into a `PanicStore`.

This is different from the more common “call a function, get a return value / error” style. Here, lifecycle is managed by:

1. **closing channels** (normal completion), and
2. **canceling contexts** (aborting / interrupting).

---

## `Async`: what it is

`Async` is a tiny building block that creates a stage with a single worker goroutine:

- it reads items from an input channel,
- applies a function `f` (1 input → 1 output),
- sends results to an output channel,
- stops when the input channel closes or when the context is canceled,
- **recovers panics** produced by `f` and stores them in a `PanicStore` found in the context (if any).

It is best seen as a streaming version of `map`.

### What `Async` guarantees

- The output channel is **always closed** when the stage finishes.
- The stage is **cancellation-aware**: it selects on `ctx.Done()` on both receive and send.
- A panic inside `f` does **not crash the process**:
    - it is recovered,
    - stored in the panic store (when present),
    - and the stage terminates by closing its output channel.

### What `Async` does *not* do

- It does **not** parallelize the mapping function across multiple goroutines.
    - Concurrency comes from **composition** (multiple stages running concurrently), or from building explicit worker pools (e.g. via `Router`).
- It does **not** magically cancel the whole pipeline on panic.
    - Panic capture is an **out-of-band signal**. The *supervisor* must decide how to react (log, cancel, re-panic, etc.).

---

## `WithPanicStore`: why it exists

Because stages run in goroutines, a panic inside a stage is not naturally returned as an error.

`WithPanicStore` attaches a write-once `PanicStore` to a context so that:

- any stage can `recover()` and store a panic,
- the pipeline supervisor can check **afterwards** whether a panic happened,
- and decide how to surface it (turn into an error, log, crash, etc.).

---

## Recommended supervision pattern

At the pipeline boundary (where you “own” execution), create a cancellable context and attach a panic store:

```go
base, cancel := context.WithCancel(context.Background())
defer cancel()

ctx, ps := textual.WithPanicStore(base)

// build and run the pipeline with ctx
out := myProcessor.Apply(ctx, in)

for v := range out {
    // consume the stream (or stop early and cancel)
}

if info, ok := ps.Load(); ok {
    // Treat this as fatal:
    // - log stack
    // - return error
    // - or re-panic
    log.Printf("panic: %v\\n%s", info.Value, info.Stack)
}
```

Key point: **you must always cancel** if you stop consuming early.

---

## The discipline required (non-negotiable rules)

This style of pipeline works extremely well when every stage follows the same contract.

### Rule 1 — Always own cancellation

If you start a pipeline, you must own a `cancel()` function and call it in all exit paths:

- normal completion,
- errors,
- “I’m done early” situations.

Use `defer cancel()` at the boundary.

### Rule 2 — Always drain or cancel

If you stop reading from a stage’s output channel before it closes:

- upstream goroutines may block on send,
- the pipeline may deadlock,
- goroutines may leak.

So if you break early, **cancel the context**.

### Rule 3 — Never close someone else’s channel

- Upstream closes the input channel.
- Downstream closes its own output channel.
- No stage should ever close an input channel it didn’t create.

### Rule 4 — Every receive and send must be cancellation-aware

Every stage must use:

```go
select {
case <-ctx.Done():
    return
case v, ok := <-in:
    ...
}
```

and similarly for sends.

`Async` enforces this pattern for the 1:1 map case.

### Rule 5 — Treat `PanicStore` as a mandatory error channel

A recovered panic is **not** automatically visible. If you forget to check the store:

- the pipeline may look like it “just ended early”,
- you can silently lose data,
- you will miss critical failures.

---

## Implementing processors and transcoders with `Async`

### 1:1 processor (S → S)

```go
upper := textual.ProcessorFunc[carrier.String](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.String {
    return textual.Async(ctx, in, func(s carrier.String) carrier.String {
        s.Value = strings.ToUpper(s.Value)
        return s
    })
})
```

### 1:1 transcoder (S1 → S2)

```go
toParcel := textual.TranscoderFunc[carrier.String, carrier.Parcel](func(ctx context.Context, in <-chan carrier.String) <-chan carrier.Parcel {
    proto := carrier.Parcel{}
    return textual.Async(ctx, in, func(s carrier.String) carrier.Parcel {
        return proto.FromUTF8String(carrier.UTF8String("P:" + s.Value)).WithIndex(s.GetIndex())
    })
})
```

### When you should *not* use `Async`

`Async` is a 1:1 mapping stage. Don’t use it if you need:

- fan-out (1 input → N outputs),
- fan-in (N inputs → 1 output),
- multi-item buffering with special flushing rules,
- reorder-by-index logic,
- streaming parsing that must keep internal state across multiple inputs.

In those cases, write a custom stage (like the processors in the existing tests) or use `Router`.

---

## Backpressure and performance

By default `Async` uses an **unbuffered output channel**:

- this keeps memory bounded,
- it makes backpressure explicit,
- it is usually the correct default for streaming text processing.

If you need more throughput, scale *explicitly*:

- use `Router` with `RoutingStrategyRoundRobin` and multiple worker processors,
- or add buffering where appropriate (at well-defined boundaries).

Remember: parallelism tends to destroy ordering. Use indices (`GetIndex`) and/or an explicit aggregation/reordering stage when order matters.

---

## Panics vs per-item errors

- **Per-item errors** (`Carrier.WithError`) are expected, recoverable, and part of the stream.
- **Panics** are unexpected faults (programmer errors, invariants violated, nil pointer deref, etc).

`Async` captures panics so you don’t lose the entire process, but you should still treat them as **fatal** at the pipeline level.

---

## Troubleshooting checklist

If a pipeline “hangs”:

- Did a consumer stop reading without canceling the context?
- Did some stage send without selecting on `ctx.Done()`?
- Did some stage forget to close its output channel?
- Did upstream forget to close the input channel?
- Did a stage panic and the supervisor never checked the PanicStore?

If outputs look truncated:

- Check `PanicStore.Load()`.
- Check per-item errors (`GetError()`).
- Ensure all outputs are drained until channels close.

---

## Where to look in the codebase

- `pkg/textual/async.go` — `Async` implementation and doc comment.
- `pkg/textual/context_with_panic_store.go` — `PanicStore` and context helpers.
- `*_test.go` files — runnable examples and unit tests.