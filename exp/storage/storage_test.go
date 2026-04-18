package storage

import (
	"errors"
	"testing"
)

func TestFor(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr error
	}{
		{"gs", "gs://bucket/key", nil},
		{"file", "file:///tmp/x", nil},
		{"unknown", "s3://bucket/key", ErrUnsupportedScheme},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := For(tc.uri)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("For(%q) unexpected error: %v", tc.uri, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("For(%q) error = %v, want %v", tc.uri, err, tc.wantErr)
			}
		})
	}
}

func TestForMemoizesPerScheme(t *testing.T) {
	a, err := For("gs://one/key")
	if err != nil {
		t.Fatal(err)
	}
	b, err := For("gs://two/other")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("expected same gcsStore for both gs:// URIs, got %p vs %p", a, b)
	}
}

func TestForParseError(t *testing.T) {
	// URL with an invalid host:port form produces a parse error.
	_, err := For("http://[::1:bad")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}
