package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestConsoleDetailsArePlainWithoutTTY(t *testing.T) {
	in := bytes.NewBuffer(nil)
	out := &bytes.Buffer{}
	console := NewConsole(in, out, &bytes.Buffer{})
	console.Details([]Detail{
		{Label: "Repository:", Value: "gix"},
		{Label: "Branch:", Value: "main"},
	})

	got := out.String()
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("expected no ANSI escapes, got %q", got)
	}
	if !strings.Contains(got, "Repository:  gix") {
		t.Fatalf("expected formatted details, got %q", got)
	}
}
