package progressbar

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteAccumulatesBytes(t *testing.T) {
	bar := DefaultBytes(100, "Test")
	bar.output = &bytes.Buffer{}

	n, err := bar.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected n=5, got %d", n)
	}
	if bar.current != 5 {
		t.Fatalf("expected current=5, got %d", bar.current)
	}

	bar.Write([]byte("world!"))
	if bar.current != 11 {
		t.Fatalf("expected current=11, got %d", bar.current)
	}
}

func TestCloseDoesNotPanic(t *testing.T) {
	bar := DefaultBytes(100, "Test")
	bar.output = &bytes.Buffer{}
	bar.Write([]byte("data"))
	if err := bar.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderContainsExpectedElements(t *testing.T) {
	var buf bytes.Buffer
	bar := DefaultBytes(1024, "Uploading")
	bar.output = &buf

	bar.Write(make([]byte, 512))
	bar.Close()

	output := buf.String()
	if !strings.Contains(output, "Uploading") {
		t.Error("output missing description")
	}
	if !strings.Contains(output, "50%") {
		t.Errorf("output missing 50%%, got: %s", output)
	}
	if !strings.Contains(output, "512 B") {
		t.Errorf("output missing byte count, got: %s", output)
	}
	if !strings.Contains(output, "1.0 KB") {
		t.Errorf("output missing total, got: %s", output)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestZeroTotal(t *testing.T) {
	var buf bytes.Buffer
	bar := DefaultBytes(0, "Empty")
	bar.output = &buf
	bar.Write([]byte("data"))
	bar.Close()

	output := buf.String()
	if !strings.Contains(output, "0%") {
		t.Errorf("expected 0%% for zero total, got: %s", output)
	}
}
