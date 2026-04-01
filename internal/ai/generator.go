package ai

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type CommitRequest struct {
	Diff             string
	Files            []string
	Language         string
	Style            string
	Type             string
	Scope            string
	WithBody         bool
	MaxSubjectLength int
	Observer         StreamObserver
}

type CommitMessage struct {
	Subject string
	Body    string
	Raw     string
	Source  string
}

type Generator interface {
	Generate(context.Context, CommitRequest) (CommitMessage, error)
}

type StreamObserver interface {
	OnReasoningDelta(string)
	OnContentDelta(string)
	OnComplete()
}

type FallbackGenerator struct{}

func (g *FallbackGenerator) Generate(_ context.Context, req CommitRequest) (CommitMessage, error) {
	commitType := firstNonEmpty(req.Type, inferType(req.Files))
	scope := firstNonEmpty(req.Scope, inferScope(req.Files))
	subject := inferSubject(req.Files, req.Language)
	if commitType == "" {
		commitType = "chore"
	}
	if scope != "" {
		subject = fmt.Sprintf("%s(%s): %s", commitType, scope, subject)
	} else {
		subject = fmt.Sprintf("%s: %s", commitType, subject)
	}
	message := CommitMessage{
		Subject: strings.TrimSpace(subject),
		Source:  "fallback",
	}
	if req.WithBody {
		body := "Summarize staged changes."
		if strings.EqualFold(req.Language, "zh") {
			body = "概述本次暂存修改。"
		}
		message.Body = body
	}
	message.Raw = joinMessage(message.Subject, message.Body)
	if req.Observer != nil {
		req.Observer.OnContentDelta(message.Raw)
		req.Observer.OnComplete()
	}
	return message, nil
}

func inferType(files []string) string {
	if len(files) == 0 {
		return "chore"
	}
	allDocs := true
	allTests := true
	for _, file := range files {
		base := strings.ToLower(filepath.Base(file))
		ext := strings.ToLower(filepath.Ext(file))
		if base != "readme.md" && ext != ".md" {
			allDocs = false
		}
		if !strings.Contains(base, "_test.") && !strings.Contains(base, ".spec.") {
			allTests = false
		}
	}
	switch {
	case allDocs:
		return "docs"
	case allTests:
		return "test"
	default:
		return "feat"
	}
}

func inferScope(files []string) string {
	if len(files) != 1 {
		return ""
	}
	parts := strings.Split(files[0], string(filepath.Separator))
	if len(parts) > 1 {
		return strings.ToLower(parts[0])
	}
	return strings.TrimSuffix(strings.ToLower(filepath.Base(files[0])), filepath.Ext(files[0]))
}

func inferSubject(files []string, language string) string {
	if strings.EqualFold(language, "zh") {
		switch len(files) {
		case 0:
			return "更新暂存修改"
		case 1:
			return fmt.Sprintf("更新 %s", filepath.Base(files[0]))
		default:
			return fmt.Sprintf("更新 %d 个文件", len(files))
		}
	}
	switch len(files) {
	case 0:
		return "update staged changes"
	case 1:
		return fmt.Sprintf("update %s", filepath.Base(files[0]))
	default:
		return fmt.Sprintf("update %d files", len(files))
	}
}

func joinMessage(subject string, body string) string {
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if body == "" {
		return subject
	}
	return subject + "\n\n" + body
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
