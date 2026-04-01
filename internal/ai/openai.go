package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type OpenAIConfig struct {
	BaseURL   string
	Model     string
	APIKeyEnv string
	Timeout   time.Duration
	Thinking  bool
}

type OpenAIGenerator struct {
	config   OpenAIConfig
	fallback Generator
	client   *http.Client
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
	apiKey := os.Getenv(g.config.APIKeyEnv)
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

	raw, err := g.readChatCompletionStream(resp, req.Observer)
	if err != nil {
		return g.fallback.Generate(ctx, req)
	}
	if raw == "" {
		return g.fallback.Generate(ctx, req)
	}

	subject, messageBody := splitAIMessage(raw)
	return CommitMessage{
		Subject: subject,
		Body:    messageBody,
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
		typeHint = fmt.Sprintf(" Prefer commit type %q.", req.Type)
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
		"You write high quality Git commit messages. Use %s style in %s. Keep the subject within %d characters. %s%s%s Do not add markdown fences or explanations.",
		style,
		language,
		req.MaxSubjectLength,
		bodyRule,
		typeHint,
		scopeHint,
	)
}

func buildUserPrompt(req CommitRequest) string {
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
