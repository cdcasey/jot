package discord

import (
	"strings"
	"testing"
)

// --- stripMention ---

func TestStripMention_Standard(t *testing.T) {
	got := stripMention("<@123456> hello", "123456")
	want := " hello"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripMention_Nickname(t *testing.T) {
	got := stripMention("<@!123456> hello", "123456")
	want := " hello"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripMention_Both(t *testing.T) {
	got := stripMention("<@123> and <@!123>", "123")
	want := " and "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStripMention_NoMention(t *testing.T) {
	got := stripMention("just text", "123")
	if got != "just text" {
		t.Errorf("got %q, want %q", got, "just text")
	}
}

func TestStripMention_WrongUser(t *testing.T) {
	input := "<@999> hello"
	got := stripMention(input, "123")
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}

func TestStripMention_Empty(t *testing.T) {
	got := stripMention("", "123")
	if got != "" {
		t.Errorf("got %q, want %q", got, "")
	}
}

// --- splitMessage ---

func TestSplitMessage_Short(t *testing.T) {
	chunks := splitMessage("hello", 2000)
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Errorf("expected single chunk 'hello', got %v", chunks)
	}
}

func TestSplitMessage_ExactLimit(t *testing.T) {
	s := strings.Repeat("a", 2000)
	chunks := splitMessage(s, 2000)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestSplitMessage_SplitsAtNewline(t *testing.T) {
	// 15 chars of "a", then newline, then 15 chars of "b" = 31 chars total
	s := strings.Repeat("a", 15) + "\n" + strings.Repeat("b", 15)
	chunks := splitMessage(s, 20)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	// First chunk should split at the newline (16 chars: 15 a's + newline)
	if chunks[0] != strings.Repeat("a", 15)+"\n" {
		t.Errorf("chunk[0] = %q", chunks[0])
	}
	if chunks[1] != strings.Repeat("b", 15) {
		t.Errorf("chunk[1] = %q", chunks[1])
	}
}

func TestSplitMessage_NoNewlineFallback(t *testing.T) {
	// No newlines — should hard-split at maxLen
	s := strings.Repeat("x", 50)
	chunks := splitMessage(s, 20)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0] != strings.Repeat("x", 20) {
		t.Errorf("chunk[0] length = %d, want 20", len(chunks[0]))
	}
	if chunks[1] != strings.Repeat("x", 20) {
		t.Errorf("chunk[1] length = %d, want 20", len(chunks[1]))
	}
	if chunks[2] != strings.Repeat("x", 10) {
		t.Errorf("chunk[2] length = %d, want 10", len(chunks[2]))
	}
}

func TestSplitMessage_Empty(t *testing.T) {
	chunks := splitMessage("", 2000)
	if len(chunks) != 1 || chunks[0] != "" {
		t.Errorf("expected single empty chunk, got %v", chunks)
	}
}

func TestSplitMessage_MultipleNewlines(t *testing.T) {
	// Should prefer the LAST newline before the limit
	s := "line1\nline2\nline3\nline4"
	chunks := splitMessage(s, 12)

	// "line1\nline2\n" is 12 chars — should split right there
	if chunks[0] != "line1\nline2\n" {
		t.Errorf("chunk[0] = %q, want %q", chunks[0], "line1\nline2\n")
	}
}
