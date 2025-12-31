package openaitextual

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/benoit-pereira-da-silva/textual/pkg/textual"
)

type OpenaiClient struct {
	config     ClientConfig
	httpClient *http.Client
}

// InputItem is the minimal "message-like" shape used in the Responses `input` array.
//
// In the API, `content` can be:
//   - a plain string, OR
//   - a structured list of content parts.
//
// We keep it as `any` so callers can provide either representation.
type InputItem struct {
	Role    string `json:"role,omitempty"`
	Content any    `json:"content,omitempty"`
}

// ResponsesRequest
// https://platform.openai.com/docs/api-reference/responses
type ResponsesRequest struct {
	// Main fields for basic support.
	//
	// We intentionally keep this minimal:
	// - no tool calling,
	// - no multimodal,
	// - no function calling.
	//
	// You can still support the conversation state by passing a full history in Input.
	Model           string `json:"model,omitempty"`
	Input           any    `json:"input,omitempty"`
	Stream          bool   `json:"stream,omitempty"`
	Instructions    string `json:"instructions,omitempty"`
	MaxOutputTokens *int   `json:"max_output_tokens,omitempty"`

	// Non-JsonCarrier / transport fields (used by gptextual helpers).
	Ctx       context.Context `json:"-"`
	SplitFunc bufio.SplitFunc `json:"-"`
}

func NewClient(config ClientConfig) OpenaiClient {
	// NOTE: http.Client.Timeout covers the whole request lifetime (including reading resp.Body).
	// For streaming requests, we rely on context cancellation instead, so Timeout is left to 0.
	return OpenaiClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 0,
		},
	}
}

func (c OpenaiClient) ensureConfig() error {
	if strings.TrimSpace(c.config.apiKey) == "" {
		return errors.New("gptextual: missing OPENAI_API_KEY")
	}
	if strings.TrimSpace(c.config.baseURL) == "" {
		return errors.New("gptextual: missing OpenAI base URL (OPENAI_API_URL)")
	}
	if strings.TrimSpace(string(c.config.model)) == "" {
		return errors.New("gptextual: missing model (OPENAI_MODEL)")
	}
	return nil
}

func (c OpenaiClient) responsesURL() (string, error) {
	if strings.TrimSpace(c.config.baseURL) == "" {
		return "", errors.New("gptextual: missing OpenAI base URL")
	}
	u, err := url.Parse(c.config.baseURL)
	if err != nil {
		return "", fmt.Errorf("gptextual: invalid base URL: %w", err)
	}
	basePath := strings.TrimSuffix(u.Path, "/")
	u.Path = basePath + "/responses"
	return u.String(), nil
}

// ResponsesStream opens a streaming connection to the Responses endpoint and returns the raw HTTP response.
// Callers must close resp.Body.
func (c OpenaiClient) ResponsesStream(ctx context.Context, r *ResponsesRequest) (*http.Response, error) {
	if err := c.ensureConfig(); err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("gptextual: nil ResponsesRequest")
	}

	// Prefer the explicit ctx, else request ctx, else background.
	if ctx == nil {
		ctx = r.Ctx
		if ctx == nil {
			ctx = context.Background()
		}
	}

	endpoint, err := c.responsesURL()
	if err != nil {
		return nil, err
	}

	// Copy so we can fill defaults without mutating the caller.
	payload := *r
	if strings.TrimSpace(payload.Model) == "" {
		payload.Model = string(c.config.model)
	}
	if payload.Input == nil {
		return nil, errors.New("gptextual: request Input is required")
	}
	// Force streaming for this helper.
	payload.Stream = true

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("gptextual: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("gptextual: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.config.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gptextual: http request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		msg := strings.TrimSpace(string(b))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("gptextual: responses stream failed: http %d: %s", resp.StatusCode, msg)
	}

	return resp, nil
}

// ProcessResponses calls the response endpoint.
//
// If an error occurs immediately, it returns an error.
// If there is an error during processing, the error is stored in the Carrier.
func ProcessResponses[S textual.Carrier[S]](c OpenaiClient, r *ResponsesRequest, processor textual.Processor[S]) error {
	if processor == nil {
		return errors.New("gptextual: nil processor")
	}
	if r == nil {
		return errors.New("gptextual: nil ResponsesRequest")
	}

	ctx := r.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	resp, err := c.ResponsesStream(ctx, r)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	ioProc := textual.NewIOReaderProcessor(processor, resp.Body)
	ioProc.SetContext(ctx)
	if r.SplitFunc != nil {
		ioProc.SetSplitFunc(r.SplitFunc)
	}

	outCh := ioProc.Start()
	for range outCh {
		// Drain to completion so the request finishes before returning.
	}
	return nil
}

// TranscodeResponses calls the response endpoint and connects the stream to a textual Transcoder.
func TranscodeResponses[S1 textual.Carrier[S1], S2 textual.Carrier[S2]](c OpenaiClient, r *ResponsesRequest, processor textual.Transcoder[S1, S2]) error {
	if processor == nil {
		return errors.New("gptextual: nil transcoder")
	}
	if r == nil {
		return errors.New("gptextual: nil ResponsesRequest")
	}

	ctx := r.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	resp, err := c.ResponsesStream(ctx, r)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	ioT := textual.NewIOReaderTranscoder(processor, resp.Body)
	ioT.SetContext(ctx)
	if r.SplitFunc != nil {
		ioT.SetSplitFunc(r.SplitFunc)
	}

	outCh := ioT.Start()
	for range outCh {
		// Drain to completion so the request finishes before returning.
	}
	return nil
}
