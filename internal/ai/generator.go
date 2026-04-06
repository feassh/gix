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
	Evidence         CommitEvidence
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
	commitType := firstNonEmpty(req.Type, inferType(req))
	scope := firstNonEmpty(req.Scope, inferScope(req))
	subjectText := inferSubject(req, scope)
	if commitType == "" {
		commitType = "chore"
	}
	var subject string
	if scope != "" {
		subject = fmt.Sprintf("%s(%s): %s", commitType, scope, subjectText)
	} else {
		subject = fmt.Sprintf("%s: %s", commitType, subjectText)
	}
	subject = truncateSubject(strings.TrimSpace(subject), req.MaxSubjectLength)

	message := CommitMessage{
		Subject: subject,
		Source:  "fallback",
	}
	if req.WithBody {
		message.Body = inferBody(req)
	}
	message.Raw = joinMessage(message.Subject, message.Body)
	if req.Observer != nil {
		req.Observer.OnContentDelta(message.Raw)
		req.Observer.OnComplete()
	}
	return message, nil
}

func inferType(req CommitRequest) string {
	if !req.Evidence.Empty() {
		counts := req.Evidence.KindCounts()
		total := len(req.Evidence.Files)
		if total == 0 {
			return "chore"
		}
		if counts["docs"] == total {
			return "docs"
		}
		if counts["test"] == total {
			return "test"
		}
		if counts["config"]+counts["build"]+counts["meta"]+counts["lockfile"] == total {
			return "chore"
		}
		if req.Evidence.Overview.Renames > 0 && req.Evidence.Overview.Additions == 0 && req.Evidence.Overview.Deletions == 0 {
			return "refactor"
		}
		sourceish := counts["source"] + counts["migration"]
		if sourceish > 0 {
			createdSource := 0
			for _, file := range req.Evidence.Files {
				if (file.Kind == "source" || file.Kind == "migration") && file.Status == "A" {
					createdSource++
				}
			}
			if createdSource > 0 && req.Evidence.Overview.Renames == 0 && req.Evidence.Overview.Deletions == 0 && req.Evidence.Overview.Additions > req.Evidence.Overview.Deletions {
				return "feat"
			}
			return "refactor"
		}
		return "chore"
	}
	return inferTypeFromFiles(req.Files)
}

func inferTypeFromFiles(files []string) string {
	if len(files) == 0 {
		return "chore"
	}
	allDocs := true
	allTests := true
	allMeta := true
	for _, file := range files {
		kind := classifyPath(file)
		if kind != "docs" {
			allDocs = false
		}
		if kind != "test" {
			allTests = false
		}
		if kind != "config" && kind != "build" && kind != "meta" && kind != "lockfile" {
			allMeta = false
		}
	}
	switch {
	case allDocs:
		return "docs"
	case allTests:
		return "test"
	case allMeta:
		return "chore"
	default:
		return "refactor"
	}
}

func inferScope(req CommitRequest) string {
	if !req.Evidence.Empty() {
		if scope := req.Evidence.DominantScope(); scope != "" {
			return scope
		}
	}
	return inferScopeFromFiles(req.Files)
}

func inferScopeFromFiles(files []string) string {
	if len(files) == 0 {
		return ""
	}
	weights := make(map[string]int)
	for _, file := range files {
		scope := scopeCandidate(file)
		if scope == "" {
			continue
		}
		weights[scope]++
	}
	bestScope := ""
	bestWeight := 0
	for scope, weight := range weights {
		if weight > bestWeight || (weight == bestWeight && (bestScope == "" || scope < bestScope)) {
			bestScope = scope
			bestWeight = weight
		}
	}
	return bestScope
}

func inferSubject(req CommitRequest, scope string) string {
	kind := dominantKind(req)
	if strings.EqualFold(req.Language, "zh") {
		switch kind {
		case "docs":
			return "更新文档"
		case "test":
			if scope != "" {
				return fmt.Sprintf("更新 %s 测试", scope)
			}
			return "更新测试"
		case "config", "build", "meta", "lockfile":
			if scope != "" {
				return fmt.Sprintf("更新 %s 配置", scope)
			}
			return "更新项目配置"
		default:
			if scope != "" {
				return fmt.Sprintf("更新 %s 相关改动", scope)
			}
			switch len(req.Files) {
			case 0:
				return "更新暂存修改"
			case 1:
				return fmt.Sprintf("更新 %s", filepath.Base(req.Files[0]))
			default:
				return fmt.Sprintf("更新 %d 个文件", len(req.Files))
			}
		}
	}

	switch kind {
	case "docs":
		return "update docs"
	case "test":
		if scope != "" {
			return fmt.Sprintf("update %s tests", scope)
		}
		return "update tests"
	case "config", "build", "meta", "lockfile":
		if scope != "" {
			return fmt.Sprintf("update %s config", scope)
		}
		return "update project config"
	default:
		if scope != "" {
			return fmt.Sprintf("update %s changes", scope)
		}
		switch len(req.Files) {
		case 0:
			return "update staged changes"
		case 1:
			return fmt.Sprintf("update %s", filepath.Base(req.Files[0]))
		default:
			return fmt.Sprintf("update %d files", len(req.Files))
		}
	}
}

func dominantKind(req CommitRequest) string {
	if !req.Evidence.Empty() {
		counts := req.Evidence.KindCounts()
		priority := map[string]int{
			"source":    6,
			"migration": 5,
			"config":    4,
			"build":     4,
			"meta":      3,
			"lockfile":  3,
			"test":      2,
			"docs":      1,
		}
		bestKind := ""
		bestCount := 0
		bestPriority := -1
		for kind, count := range counts {
			kindPriority := priority[kind]
			if count > bestCount || (count == bestCount && kindPriority > bestPriority) || (count == bestCount && kindPriority == bestPriority && (bestKind == "" || kind < bestKind)) {
				bestKind = kind
				bestCount = count
				bestPriority = kindPriority
			}
		}
		return bestKind
	}
	if len(req.Files) == 1 {
		return classifyPath(req.Files[0])
	}
	return "source"
}

func inferBody(req CommitRequest) string {
	if strings.EqualFold(req.Language, "zh") {
		if !req.Evidence.Empty() {
			parts := []string{fmt.Sprintf("涉及 %d 个文件", req.Evidence.Overview.FileCount)}
			if scope := req.Evidence.DominantScope(); scope != "" {
				parts = append(parts, fmt.Sprintf("主要集中在 %s", scope))
			}
			return strings.Join(parts, "，") + "。"
		}
		return "概述本次暂存修改。"
	}
	if !req.Evidence.Empty() {
		parts := []string{fmt.Sprintf("Touch %d files", req.Evidence.Overview.FileCount)}
		if scope := req.Evidence.DominantScope(); scope != "" {
			parts = append(parts, "mainly in "+scope)
		}
		return strings.Join(parts, ", ") + "."
	}
	return "Summarize staged changes."
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

func truncateSubject(subject string, maxLen int) string {
	subject = strings.TrimSpace(subject)
	if maxLen <= 0 {
		maxLen = 72
	}
	runes := []rune(subject)
	if len(runes) <= maxLen {
		return subject
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return strings.TrimSpace(string(runes[:maxLen-1])) + "…"
}
