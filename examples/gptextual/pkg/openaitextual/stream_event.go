package openaitextual

/*
StreamEventType represents the semantic event types emitted by the OpenAI
Responses API when streaming is enabled.

Each streamed message is a JSON object with a `type` field and optional
payload fields depending on the event.

The stream is ordered and SHOULD be processed sequentially.
*/
type StreamEventType string

const (
	// ─────────────────────────────────────────────────────────────
	// Lifecycle events
	// ─────────────────────────────────────────────────────────────

	// StreamEventResponseCreated is emitted once when the response object
	// has been created and inference begins.
	StreamEventResponseCreated StreamEventType = "response.created"

	// StreamEventResponseInProgress is emitted while the model is actively
	// generating output.
	StreamEventResponseInProgress StreamEventType = "response.in_progress"

	// StreamEventResponseCompleted signals that the response stream has
	// successfully finished.
	StreamEventResponseCompleted StreamEventType = "response.completed"

	// StreamEventResponseFailed indicates that response generation failed.
	StreamEventResponseFailed StreamEventType = "response.failed"

	// ─────────────────────────────────────────────────────────────
	// Text output events
	// ─────────────────────────────────────────────────────────────

	// StreamEventOutputTextDelta contains an incremental chunk of generated text.
	// The `Delta` field will be populated.
	StreamEventOutputTextDelta StreamEventType = "response.output_text.delta"

	// StreamEventTextDone indicates that all text output has been streamed.
	StreamEventTextDone StreamEventType = "response.text.done"

	// StreamEventOutputTextAnnotationAdded carries metadata or annotations
	// associated with the generated text.
	StreamEventOutputTextAnnotationAdded StreamEventType = "response.output_text_annotation_added"

	// ─────────────────────────────────────────────────────────────
	// Structured output events
	// ─────────────────────────────────────────────────────────────

	// StreamEventOutputItemAdded signals that a structured output item
	// (e.g. tool call, message block) has been added.
	StreamEventOutputItemAdded StreamEventType = "response.output_item_added"

	// StreamEventOutputItemDone indicates that the structured output item
	// has completed.
	StreamEventOutputItemDone StreamEventType = "response.output_item_done"

	// ─────────────────────────────────────────────────────────────
	// Function / tool call events
	// ─────────────────────────────────────────────────────────────

	// StreamEventFunctionCallArgumentsDelta streams incremental JSON
	// arguments for a function or tool call.
	StreamEventFunctionCallArgumentsDelta StreamEventType = "response.function_call_arguments.delta"

	// StreamEventFunctionCallArgumentsDone signals that the function
	// call arguments are fully streamed.
	StreamEventFunctionCallArgumentsDone StreamEventType = "response.function_call_arguments.done"

	// ─────────────────────────────────────────────────────────────
	// Code Interpreter events
	// ─────────────────────────────────────────────────────────────

	// StreamEventCodeInterpreterInProgress indicates that the code
	// interpreter tool is executing.
	StreamEventCodeInterpreterInProgress StreamEventType = "response.code_interpreter_in_progress"

	// StreamEventCodeInterpreterCallCodeDelta streams code being executed
	// by the interpreter.
	StreamEventCodeInterpreterCallCodeDelta StreamEventType = "response.code_interpreter_call_code_delta"

	// StreamEventCodeInterpreterCallCodeDone signals that code streaming
	// has completed.
	StreamEventCodeInterpreterCallCodeDone StreamEventType = "response.code_interpreter_call_code_done"

	// StreamEventCodeInterpreterCallInterpreting indicates that the
	// interpreter is evaluating results.
	StreamEventCodeInterpreterCallInterpreting StreamEventType = "response.code_interpreter_call_interpreting"

	// StreamEventCodeInterpreterCallCompleted indicates the interpreter
	// call has fully completed.
	StreamEventCodeInterpreterCallCompleted StreamEventType = "response.code_interpreter_call_completed"

	// ─────────────────────────────────────────────────────────────
	// File search events
	// ─────────────────────────────────────────────────────────────

	// StreamEventFileSearchCallInProgress indicates a file search tool
	// invocation has started.
	StreamEventFileSearchCallInProgress StreamEventType = "response.file_search_call_in_progress"

	// StreamEventFileSearchCallSearching indicates that file search
	// is actively querying sources.
	StreamEventFileSearchCallSearching StreamEventType = "response.file_search_call_searching"

	// StreamEventFileSearchCallCompleted indicates the file search
	// tool has completed.
	StreamEventFileSearchCallCompleted StreamEventType = "response.file_search_call_completed"

	// ─────────────────────────────────────────────────────────────
	// Refusal & error events
	// ─────────────────────────────────────────────────────────────

	// StreamEventRefusalDelta streams partial refusal content.
	StreamEventRefusalDelta StreamEventType = "response.refusal.delta"

	// StreamEventRefusalDone indicates refusal streaming has finished.
	StreamEventRefusalDone StreamEventType = "response.refusal.done"

	// StreamEventError represents a generic stream error.
	StreamEventError StreamEventType = "error"
)

/*
StreamEvent represents a single event emitted from the Responses API
stream when `stream=true` is enabled.

Only a subset of fields will be populated depending on the event `Type`.

Field semantics:
  - Type: The event type identifier (always present)
  - Delta: Incremental text or JSON fragment
  - Text: Full text payload (non-streamed events)
  - Code: Code being executed (code interpreter events)
  - Message: Error or informational message
*/
type StreamEvent struct {
	Type    StreamEventType `json:"type"`
	Delta   string          `json:"delta,omitempty"`
	Text    string          `json:"text,omitempty"`
	Code    string          `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
}

/*
IsTerminal returns true if this event represents a terminal state
for the stream (completed, failed, or error).
*/
func (s StreamEvent) IsTerminal() bool {
	switch s.Type {
	case StreamEventResponseCompleted,
		StreamEventResponseFailed,
		StreamEventError:
		return true
	default:
		return false
	}
}

/*
IsTextDelta returns true if the event carries incremental text output.
*/
func (s StreamEvent) IsTextDelta() bool {
	return s.Type == StreamEventOutputTextDelta
}
