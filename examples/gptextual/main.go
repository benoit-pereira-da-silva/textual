package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/benoit-pereira-da-silva/textual/examples/gptextual/pkg/openaitextual"
	"github.com/benoit-pereira-da-silva/textual/pkg/textual"
)

func main() {
	var (
		modelFlag            = flag.String("model", "", "OpenAI model (overrides OPENAI_MODEL)")
		baseURLFlag          = flag.String("base-url", "", "OpenAI base URL, e.g. https://api.openai.com/v1 (overrides OPENAI_API_URL)")
		maxOutputTokensFlag  = flag.Int("max-output-tokens", 256, "Maximum output tokens (0 = omit)")
		instructionsFlag     = flag.String("instructions", "", "Optional assistant instructions (system prompt)")
		nonInteractivePrompt = flag.String("prompt", "", "If set, runs a single request and exits (otherwise starts a tiny REPL)")
	)
	flag.Parse()

	cfg := openaitextual.NewClientConfig("", *baseURLFlag, openaitextual.Model(*modelFlag))
	client := openaitextual.NewClient(cfg)

	// Ctrl-C cancellation.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// One-shot mode.
	if strings.TrimSpace(*nonInteractivePrompt) != "" {
		if err := runOnce(ctx, client, *maxOutputTokensFlag, *instructionsFlag, *nonInteractivePrompt); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// If no -prompt was provided but args exist, treat them as a one-shot prompt.
	if argPrompt := strings.TrimSpace(strings.Join(flag.Args(), " ")); argPrompt != "" {
		if err := runOnce(ctx, client, *maxOutputTokensFlag, *instructionsFlag, argPrompt); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Minimal REPL: keeps conversation history in memory.
	_, _ = fmt.Fprintln(os.Stderr, "gptextual: enter a prompt and press Enter (Ctrl-D to quit, Ctrl-C to interrupt).")
	scanner := bufio.NewScanner(os.Stdin)
	history := make([]openaitextual.InputItem, 0, 16)

	for {
		select {
		case <-ctx.Done():
			_, _ = fmt.Fprintln(os.Stderr, "\ninterrupted")
			return
		default:
		}

		_, _ = fmt.Fprint(os.Stderr, "> ")
		if !scanner.Scan() {
			// EOF or error.
			if err := scanner.Err(); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, "\nstdin error:", err)
			}
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Add user turn.
		history = append(history, openaitextual.InputItem{Role: "user", Content: line})

		// Stream assistant response and append it to history.
		assistantText, err := streamAssistant(ctx, client, *maxOutputTokensFlag, *instructionsFlag, history)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "\nerror:", err)
			continue
		}

		_, _ = fmt.Fprint(os.Stdout, "\n")
		history = append(history, openaitextual.InputItem{Role: "assistant", Content: assistantText})
	}
}

func runOnce(ctx context.Context, client openaitextual.OpenaiClient, maxOutputTokens int, instructions string, prompt string) error {
	history := []openaitextual.InputItem{{Role: "user", Content: prompt}}
	_, err := streamAssistant(ctx, client, maxOutputTokens, instructions, history)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(os.Stdout, "\n")
	return nil
}

func streamAssistant(ctx context.Context, client openaitextual.OpenaiClient, maxOutputTokens int, instructions string, history []openaitextual.InputItem) (string, error) {
	req := &openaitextual.ResponsesRequest{
		Input: history,
		//  textual.ScanJSON splits the SSE events and extracts the JsonCarrier.
		SplitFunc: textual.ScanJSON,
	}
	if instructions != "" {
		req.Instructions = instructions
	}
	if maxOutputTokens > 0 {
		mot := maxOutputTokens
		req.MaxOutputTokens = &mot
	}
	resp, err := client.ResponsesStream(ctx, req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	// Apply the transcoder func to the body split by SSE event.
	ioT := textual.NewIOReaderTranscoder(transcoder, resp.Body)
	ioT.SetContext(ctx)
	outCh := ioT.Start()

	var b strings.Builder
	for item := range outCh {
		if gErr := item.GetError(); gErr != nil {
			// Keep the stream alive, but surface errors.
			_, _ = fmt.Fprintln(os.Stderr, "\nstream error:", gErr)
			continue
		}
		b.WriteString(item.Value)
		_, _ = fmt.Fprint(os.Stdout, item.Value)
	}

	return b.String(), nil
}

var transcoder = textual.TranscoderFunc[textual.JsonGenericCarrier[openaitextual.StreamEvent], textual.StringCarrier](
	func(ctx context.Context, in <-chan textual.JsonGenericCarrier[openaitextual.StreamEvent]) <-chan textual.StringCarrier {
		return textual.AsyncEmitter(ctx, in, func(ctx context.Context, c textual.JsonGenericCarrier[openaitextual.StreamEvent], emit func(s textual.StringCarrier)) {
			ev := c.Value
			switch ev.Type {

			// ─────────────────────────────────────────────────────
			// Lifecycle events
			// ─────────────────────────────────────────────────────

			case "response.created":
				// Response object created; no textual payload to emit yet.

			case "response.in_progress":
				// Model is generating output; informational only.

			case "response.completed":
				// Entire response lifecycle is complete; channel will close soon.

			case "response.failed":
				// Response failed; error details may appear elsewhere.

			// ─────────────────────────────────────────────────────
			// Text output events
			// ─────────────────────────────────────────────────────

			case "response.output_text.delta":
				// Incremental text chunk; Text or Delta may be populated
				// depending on upstream normalization.
				emit(textual.StringFrom(ev.Text))

			case "response.text.done":
				// Text channel is complete; other response events may still follow.

			case "response.output_text_annotation_added":
				// Text metadata / annotation; not part of user-visible text.

			// ─────────────────────────────────────────────────────
			// Structured output events
			// ─────────────────────────────────────────────────────

			case "response.output_item_added":
				// Structured output item (tool call, block, etc.) added.

			case "response.output_item_done":
				// Structured output item fully emitted.

			// ─────────────────────────────────────────────────────
			// Function / tool call events
			// ─────────────────────────────────────────────────────

			case "response.function_call_arguments.delta":
				// Incremental function-call arguments (JsonCarrier); ignored here.

			case "response.function_call_arguments.done":
				// Function-call arguments completed.

			// ─────────────────────────────────────────────────────
			// Code interpreter events
			// ─────────────────────────────────────────────────────

			case "response.code_interpreter_in_progress":
				// Code interpreter has started execution.

			case "response.code_interpreter_call_code_delta":
				// Incremental code being executed by the interpreter.

			case "response.code_interpreter_call_code_done":
				// Code emission complete.

			case "response.code_interpreter_call_interpreting":
				// Interpreter evaluating results.

			case "response.code_interpreter_call_completed":
				// Interpreter execution fully completed.

			// ─────────────────────────────────────────────────────
			// File search events
			// ─────────────────────────────────────────────────────

			case "response.file_search_call_in_progress":
				// File search tool invocation started.

			case "response.file_search_call_searching":
				// File search actively querying sources.

			case "response.file_search_call_completed":
				// File search completed.

			// ─────────────────────────────────────────────────────
			// Refusal & error events
			// ─────────────────────────────────────────────────────

			case "response.refusal.delta":
				// Partial refusal message; intentionally ignored.

			case "response.refusal.done":
				// Refusal message complete.

			case "error":
				// Stream-level error event; handled upstream or via context.

			default:
				// Unknown or future event type; safely ignored for forward compatibility.
			}
		},
		)
	},
)
