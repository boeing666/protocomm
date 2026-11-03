package prototrace_test

import (
	"strings"
	"testing"

	mathpb "github.com/boeing666/protocomm/go/proto/math"
	"github.com/boeing666/protocomm/go/proto/messaging"
	. "github.com/boeing666/protocomm/go/prototrace"
)

func TestRenderNil(t *testing.T) {
	if got := Render(nil); got != "" {
		t.Errorf("Render(nil) = %q, want empty", got)
	}
	var typedNil *mathpb.CalcRequest
	if got := Render(typedNil); got != "" {
		t.Errorf("Render(typed nil) = %q, want empty", got)
	}
}

func TestRenderSingleLine(t *testing.T) {
	got := Render(&mathpb.CalcRequest{A: 2, B: 3})
	if !strings.Contains(got, "a:2") || !strings.Contains(got, "b:3") {
		t.Errorf("Render = %q, want fields a and b", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("Render produced multi-line output: %q", got)
	}
}

func TestRenderEmptyMessage(t *testing.T) {
	if got := Render(&mathpb.CalcRequest{}); got != "" {
		t.Errorf("Render(empty) = %q, want empty", got)
	}
}

func TestRenderTruncatesLongMessages(t *testing.T) {
	msg := &messaging.ChatMessage{Text: strings.Repeat("x", 2*MaxBody)}
	got := Render(msg)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("long message not marked as truncated: ...%q", got[len(got)-10:])
	}
	if body := strings.TrimSuffix(got, "…"); len(body) != MaxBody {
		t.Errorf("truncated body length = %d, want %d", len(body), MaxBody)
	}
}

func TestRenderKeepsShortMessages(t *testing.T) {
	msg := &messaging.ChatMessage{Text: "hi"}
	if got := Render(msg); strings.HasSuffix(got, "…") {
		t.Errorf("short message truncated: %q", got)
	}
}
