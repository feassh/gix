package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OpenAIConfig struct {
	BaseURL  string
	Model    string
	APIKey   string
	Timeout  time.Duration
	Thinking bool
}

type OpenAIGenerator struct {
	config   OpenAIConfig
	fallback Generator
	client   *http.Client
}

type observedStream struct {
	inner        StreamObserver
	sawReasoning bool
	sawContent   bool
}

func NewOpenAIGenerator(cfg OpenAIConfig, fallback Generator) *OpenAIGenerator {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	return &OpenAIGenerator{
		config:   cfg,
		fallback: fallback,
		client:   &http.Client{Timeout: timeout},
	}
}

func (g *OpenAIGenerator) Generate(ctx context.Context, req CommitRequest) (CommitMessage, error) {
	apiKey := strings.TrimSpace(g.config.APIKey)
	if strings.TrimSpace(apiKey) == "" {
		return g.fallback.Generate(ctx, req)
	}

	body := map[string]any{
		"model": g.config.Model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": buildSystemPrompt(req),
			},
			{
				"role":    "user",
				"content": buildUserPrompt(req),
			},
		},
		"stream":           true,
		"reasoning_effort": reasoningEffort(g.config.Thinking),
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return CommitMessage{}, err
	}

	endpoint, err := chatCompletionsURL(g.config.BaseURL)
	if err != nil {
		return g.fallback.Generate(ctx, req)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return CommitMessage{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return g.fallback.Generate(ctx, req)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return g.fallback.Generate(ctx, req)
	}

	observer := &observedStream{inner: req.Observer}
	raw, err := g.readChatCompletionStream(resp, observer)
	if err != nil {
		if observer.started() {
			return CommitMessage{}, fmt.Errorf("ai stream interrupted: %w", err)
		}
		return g.fallback.Generate(ctx, req)
	}
	if raw == "" {
		if observer.started() {
			return CommitMessage{}, fmt.Errorf("ai stream ended without message content")
		}
		return g.fallback.Generate(ctx, req)
	}

	raw = stripMarkdownFences(raw)
	subject, messageBody := splitAIMessage(raw)
	subject = truncateSubject(subject, req.MaxSubjectLength)
	return CommitMessage{
		Subject: subject,
		Body:    strings.TrimSpace(messageBody),
		Raw:     joinMessage(subject, messageBody),
		Source:  "ai",
	}, nil
}

func chatCompletionsURL(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid ai.base_url: %s", strings.TrimSpace(base))
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	return parsed.String(), nil
}

func reasoningEffort(enabled bool) string {
	if enabled {
		return "high"
	}
	return "none"
}

func (g *OpenAIGenerator) readChatCompletionStream(resp *http.Response, observer StreamObserver) (string, error) {
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return readChatCompletionSSE(resp.Body, observer)
	}
	return readChatCompletionJSON(resp.Body, observer)
}

func (o *observedStream) OnReasoningDelta(delta string) {
	if delta != "" {
		o.sawReasoning = true
	}
	if o.inner != nil {
		o.inner.OnReasoningDelta(delta)
	}
}

func (o *observedStream) OnContentDelta(delta string) {
	if delta != "" {
		o.sawContent = true
	}
	if o.inner != nil {
		o.inner.OnContentDelta(delta)
	}
}

func (o *observedStream) OnComplete() {
	if o.inner != nil {
		o.inner.OnComplete()
	}
}

func (o *observedStream) started() bool {
	return o.sawReasoning || o.sawContent
}

func buildSystemPrompt(req CommitRequest) string {
	language := "English"
	if strings.EqualFold(req.Language, "zh") {
		language = "Chinese"
	}
	bodyRule := "Return only a commit subject."
	if req.WithBody {
		bodyRule = "Return a commit subject, a blank line, and a concise body."
	}
	typeHint := ""
	if strings.TrimSpace(req.Type) != "" {
		typeHint = fmt.Sprintf(" Prefer commit type %q only if the evidence clearly supports it.", req.Type)
	}
	scopeHint := ""
	if strings.TrimSpace(req.Scope) != "" {
		scopeHint = fmt.Sprintf(" Use scope %q.", req.Scope)
	}
	style := req.Style
	if style == "" {
		style = "conventional"
	}
	return fmt.Sprintf(
		"You write high quality Git commit messages. Use %s style in %s. Keep the subject within %d characters. %s Use only the provided evidence. Do not invent features, bug fixes, behavior changes, or root causes. When the evidence is ambiguous or partial, prefer a broader summary. Prefer conservative commit types like refactor or chore when intent is unclear.%s%s Do not add markdown fences or explanations.",
		style,
		language,
		requiredSubjectLength(req.MaxSubjectLength),
		bodyRule,
		typeHint,
		scopeHint,
	)
}

func buildUserPrompt(req CommitRequest) string {
	if prompt := strings.TrimSpace(req.Evidence.Prompt(defaultEvidenceBudgetChars)); prompt != "" {
		return prompt + "\n\nWrite the commit message from this evidence only. If some details are missing, stay generic instead of guessing."
	}
	return fmt.Sprintf("Changed files:\n%s\n\nStaged diff:\n%s", strings.Join(req.Files, "\n"), req.Diff)
}

func splitAIMessage(raw string) (string, string) {
	clean := strings.TrimSpace(raw)
	parts := strings.SplitN(clean, "\n\n", 2)
	subject := strings.TrimSpace(parts[0])
	body := ""
	if len(parts) == 2 {
		body = strings.TrimSpace(parts[1])
	}
	return subject, body
}

func stripMarkdownFences(raw string) string {
	clean := strings.TrimSpace(raw)
	if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```")
		if idx := strings.Index(clean, "\n"); idx >= 0 {
			clean = clean[idx+1:]
		}
	}
	if strings.HasSuffix(clean, "```") {
		clean = strings.TrimSuffix(clean, "```")
	}
	return strings.TrimSpace(clean)
}

func requiredSubjectLength(length int) int {
	if length <= 0 {
		return 72
	}
	return length
}
