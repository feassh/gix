package ai

import (
	"strings"
	"testing"
)

type testObserver struct {
	reasoning strings.Builder
	content   strings.Builder
	done      bool
}

func (o *testObserver) OnReasoningDelta(delta string) {
	o.reasoning.WriteString(delta)
}

func (o *testObserver) OnContentDelta(delta string) {
	o.content.WriteString(delta)
}

func (o *testObserver) OnComplete() {
	o.done = true
}

func TestReadChatCompletionSSEWithReasoning(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"Analyzing diff..."}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"feat: improve cli output"}}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n"))

	observer := &testObserver{}
	got, err := readChatCompletionSSE(stream, observer)
	if err != nil {
		t.Fatalf("readChatCompletionSSE() error = %v", err)
	}
	if got != "feat: improve cli output" {
		t.Fatalf("content = %q", got)
	}
	if observer.reasoning.String() != "Analyzing diff..." {
		t.Fatalf("reasoning = %q", observer.reasoning.String())
	}
	if observer.content.String() != "feat: improve cli output" {
		t.Fatalf("observer content = %q", observer.content.String())
	}
	if !observer.done {
		t.Fatalf("expected observer done")
	}
}

func TestChatCompletionsURLNormalizesV1(t *testing.T) {
	got, err := chatCompletionsURL("https://example.com/v1")
	if err != nil {
		t.Fatalf("chatCompletionsURL() error = %v", err)
	}
	want := "https://example.com/v1/chat/completions"
	if got != want {
		t.Fatalf("chatCompletionsURL() = %q, want %q", got, want)
	}
}

func TestChatCompletionsURLKeepsCustomPath(t *testing.T) {
	got, err := chatCompletionsURL("https://example.com/openai/v1")
	if err != nil {
		t.Fatalf("chatCompletionsURL() error = %v", err)
	}
	want := "https://example.com/openai/v1/chat/completions"
	if got != want {
		t.Fatalf("chatCompletionsURL() = %q, want %q", got, want)
	}
}

func TestReasoningEffort(t *testing.T) {
	if got := reasoningEffort(true); got != "high" {
		t.Fatalf("reasoningEffort(true) = %q", got)
	}
	if got := reasoningEffort(false); got != "none" {
		t.Fatalf("reasoningEffort(false) = %q", got)
	}
}

func TestReadChatCompletionSSEWithMidStreamError(t *testing.T) {
	stream := strings.NewReader(strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning":"Thinking..."}}]}`,
		"",
		`data: {"error":{"code":"server_error","message":"Provider disconnected unexpectedly"},"choices":[{"delta":{"content":""},"finish_reason":"error"}]}`,
		"",
	}, "\n"))

	observer := &testObserver{}
	_, err := readChatCompletionSSE(stream, observer)
	if err == nil || !strings.Contains(err.Error(), "Provider disconnected unexpectedly") {
		t.Fatalf("expected mid-stream error, got %v", err)
	}
	if observer.reasoning.String() != "Thinking..." {
		t.Fatalf("reasoning = %q", observer.reasoning.String())
	}
}
