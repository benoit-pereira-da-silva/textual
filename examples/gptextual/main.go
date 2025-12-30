package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/benoit-pereira-da-silva/textual/examples/gptextual/pkg/gptextual"
	"github.com/benoit-pereira-da-silva/textual/pkg/carrier"
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

	cfg := gptextual.NewClientConfig("", *baseURLFlag, gptextual.Model(*modelFlag))
	client := gptextual.NewClient(cfg)

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
	history := make([]gptextual.InputItem, 0, 16)

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
		history = append(history, gptextual.InputItem{Role: "user", Content: line})

		// Stream assistant response and append it to history.
		assistantText, err := streamAssistant(ctx, client, *maxOutputTokensFlag, *instructionsFlag, history)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "\nerror:", err)
			continue
		}

		_, _ = fmt.Fprint(os.Stdout, "\n")
		history = append(history, gptextual.InputItem{Role: "assistant", Content: assistantText})
	}
}

func runOnce(ctx context.Context, client gptextual.OpenaiClient, maxOutputTokens int, instructions string, prompt string) error {
	history := []gptextual.InputItem{{Role: "user", Content: prompt}}
	_, err := streamAssistant(ctx, client, maxOutputTokens, instructions, history)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(os.Stdout, "\n")
	return nil
}

func streamAssistant(ctx context.Context, client gptextual.OpenaiClient, maxOutputTokens int, instructions string, history []gptextual.InputItem) (string, error) {
	req := &gptextual.ResponsesRequest{
		Input:     history,
		SplitFunc: textual.ScanJSON, // Extracts the JSON from the SSE events.
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

var transcoder = textual.TranscoderFunc[gptextual.StreamEvent, carrier.String](func(ctx context.Context, in <-chan gptextual.StreamEvent) <-chan carrier.String {
	return textual.AsyncEmitter(ctx, in, func(ctx context.Context, ev gptextual.StreamEvent, emit func(s carrier.String)) {

		// That's

		emit(carrier.StringFrom(ev.Text))
	})
})
