package ai

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessageDelta `json:"message"`
	} `json:"choices"`
}

type chatCompletionChunk struct {
	Error   *chatStreamError `json:"error"`
	Choices []struct {
		Delta        chatMessageDelta `json:"delta"`
		FinishReason string           `json:"finish_reason"`
	} `json:"choices"`
}

type chatStreamError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type chatMessageDelta struct {
	Content          string `json:"content"`
	Reasoning        string `json:"reasoning"`
	ReasoningContent string `json:"reasoning_content"`
}

func readChatCompletionJSON(body io.Reader, observer StreamObserver) (string, error) {
	var parsed chatCompletionResponse
	if err := json.NewDecoder(body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("empty chat completion response")
	}
	if observer != nil {
		if parsed.Choices[0].Message.ReasoningContent != "" {
			observer.OnReasoningDelta(parsed.Choices[0].Message.ReasoningContent)
		} else if parsed.Choices[0].Message.Reasoning != "" {
			observer.OnReasoningDelta(parsed.Choices[0].Message.Reasoning)
		}
		if parsed.Choices[0].Message.Content != "" {
			observer.OnContentDelta(parsed.Choices[0].Message.Content)
		}
		observer.OnComplete()
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func readChatCompletionSSE(body io.Reader, observer StreamObserver) (string, error) {
	reader := bufio.NewReader(body)
	var (
		eventLines []string
		content    strings.Builder
	)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if done, err := handleSSEEvent(eventLines, &content, observer); err != nil {
				return "", err
			} else if done {
				if observer != nil {
					observer.OnComplete()
				}
				return strings.TrimSpace(content.String()), nil
			}
			eventLines = eventLines[:0]
		} else {
			eventLines = append(eventLines, line)
		}
		if errors.Is(err, io.EOF) {
			if len(eventLines) > 0 {
				if done, err := handleSSEEvent(eventLines, &content, observer); err != nil {
					return "", err
				} else if done {
					if observer != nil {
						observer.OnComplete()
					}
					return strings.TrimSpace(content.String()), nil
				}
			}
			if observer != nil {
				observer.OnComplete()
			}
			return strings.TrimSpace(content.String()), nil
		}
	}
}

func handleSSEEvent(lines []string, content *strings.Builder, observer StreamObserver) (bool, error) {
	if len(lines) == 0 {
		return false, nil
	}
	var payloadParts []string
	for _, line := range lines {
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			payloadParts = append(payloadParts, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if len(payloadParts) == 0 {
		return false, nil
	}
	payload := strings.Join(payloadParts, "\n")
	if payload == "[DONE]" {
		return true, nil
	}
	var chunk chatCompletionChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return false, err
	}
	if chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
		return false, errors.New(strings.TrimSpace(chunk.Error.Message))
	}
	for _, choice := range chunk.Choices {
		if choice.FinishReason == "error" && chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
			return false, errors.New(strings.TrimSpace(chunk.Error.Message))
		}
		reasoning := firstNonEmpty(choice.Delta.ReasoningContent, choice.Delta.Reasoning)
		if observer != nil && reasoning != "" {
			observer.OnReasoningDelta(reasoning)
		}
		if choice.Delta.Content != "" {
			content.WriteString(choice.Delta.Content)
			if observer != nil {
				observer.OnContentDelta(choice.Delta.Content)
			}
		}
	}
	return false, nil
}
