package progressbar

import (
	"bytes"
	"strings"
	"testing"
	"time"
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

func TestSetUpdatesProgress(t *testing.T) {
	bar := DefaultCount(100, "Items")
	bar.output = &bytes.Buffer{}

	bar.Set(42)
	if bar.current != 42 {
		t.Fatalf("expected current=42, got %d", bar.current)
	}

	bar.Set(99)
	if bar.current != 99 {
		t.Fatalf("expected current=99, got %d", bar.current)
	}
}

func TestAddIncrementsProgress(t *testing.T) {
	bar := DefaultCount(100, "Items")
	bar.output = &bytes.Buffer{}

	got := bar.Add(10)
	if got != 10 {
		t.Fatalf("expected Add to return 10, got %d", got)
	}

	got = bar.Add(5)
	if got != 15 {
		t.Fatalf("expected Add to return 15, got %d", got)
	}

	if bar.current != 15 {
		t.Fatalf("expected current=15, got %d", bar.current)
	}
}

func TestCountRenderFormat(t *testing.T) {
	var buf bytes.Buffer
	bar := DefaultCount(200, "Processing")
	bar.output = &buf

	bar.Set(100)
	bar.Close()

	output := buf.String()
	if !strings.Contains(output, "Processing") {
		t.Error("output missing description")
	}
	if !strings.Contains(output, "50%") {
		t.Errorf("output missing 50%%, got: %s", output)
	}
	if !strings.Contains(output, "100 / 200") {
		t.Errorf("output missing count format, got: %s", output)
	}
}

func TestBlockCharsOption(t *testing.T) {
	var buf bytes.Buffer
	bar := DefaultBytes(100, "Upload", WithBlockChars())
	bar.output = &buf

	bar.Write(make([]byte, 50))
	bar.Close()

	output := buf.String()
	if !strings.Contains(output, "\u2588") {
		t.Errorf("expected block chars in output, got: %s", output)
	}
}

func TestSpeedDisplay(t *testing.T) {
	var buf bytes.Buffer
	bar := DefaultBytes(1024*1024, "Download", WithSpeed())
	bar.output = &buf
	bar.startTime = time.Now().Add(-1 * time.Second)

	bar.Write(make([]byte, 512*1024))
	bar.Close()

	output := buf.String()
	if !strings.Contains(output, "/s") {
		t.Errorf("expected speed in output, got: %s", output)
	}
	if !strings.Contains(output, "(") {
		t.Errorf("expected parentheses with speed option, got: %s", output)
	}
}

func TestETADisplay(t *testing.T) {
	var buf bytes.Buffer
	bar := DefaultBytes(1024, "Upload", WithETA())
	bar.output = &buf
	bar.startTime = time.Now().Add(-5 * time.Second)

	bar.Write(make([]byte, 512))
	bar.Close()

	output := buf.String()
	if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
		t.Errorf("expected ETA brackets in output, got: %s", output)
	}
}

func TestETAHiddenAtComplete(t *testing.T) {
	var buf bytes.Buffer
	bar := DefaultBytes(100, "Upload", WithETA())
	bar.output = &buf

	bar.Write(make([]byte, 100))
	bar.Close()

	output := buf.String()
	if strings.Contains(output, "[") && strings.Contains(output, "]") {
		t.Errorf("expected no ETA at completion, got: %s", output)
	}
}

func TestFormatCount(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{42, "42"},
		{1000, "1000"},
		{999999, "999999"},
	}
	for _, tt := range tests {
		got := formatCount(tt.input)
		if got != tt.expected {
			t.Errorf("formatCount(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0s"},
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m0s"},
		{90 * time.Second, "1m30s"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.input)
		if got != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestWithWidthOption(t *testing.T) {
	bar := DefaultCount(100, "Test", WithWidth(80))
	if bar.width != 80 {
		t.Fatalf("expected width=80, got %d", bar.width)
	}
}

func TestWithOutputOption(t *testing.T) {
	var buf bytes.Buffer
	bar := DefaultCount(100, "Test", WithOutput(&buf))
	bar.Set(50)
	bar.Close()
	if buf.Len() == 0 {
		t.Fatal("expected output to be written to custom writer")
	}
}
