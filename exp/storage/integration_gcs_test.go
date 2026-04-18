//go:build integration

package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func newKeyPrefix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "exp-storage-it/" + time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

func TestIntegrationGCSRoundTrip(t *testing.T) {
	bucket := os.Getenv("STORAGE_TEST_BUCKET")
	if bucket == "" {
		t.Skip("STORAGE_TEST_BUCKET not set")
	}
	ctx := context.Background()
	prefix := newKeyPrefix(t)
	uri := "gs://" + bucket + "/" + prefix + "/hello.txt"

	t.Cleanup(func() {
		_ = Delete(ctx, uri)
	})

	payload := []byte("hello from exp/storage integration test")
	if err := PutBytes(ctx, uri, payload, "text/plain"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}

	got, err := Get(ctx, uri)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", got, payload)
	}

	objs, err := List(ctx, "gs://"+bucket+"/"+prefix+"/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var found bool
	for _, o := range objs {
		if strings.HasSuffix(o.URI, "/hello.txt") {
			found = true
			if o.ContentType != "text/plain" {
				t.Errorf("ContentType = %q, want text/plain", o.ContentType)
			}
			if o.Size != int64(len(payload)) {
				t.Errorf("Size = %d, want %d", o.Size, len(payload))
			}
			if o.Generation == 0 {
				t.Errorf("Generation = 0, want non-zero from GCS")
			}
			if o.Metageneration == 0 {
				t.Errorf("Metageneration = 0, want non-zero from GCS")
			}
		}
	}
	if !found {
		t.Fatalf("List returned %d objects, none matched our key", len(objs))
	}

	if err := Delete(ctx, uri); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = Get(ctx, uri)
	if err == nil {
		t.Fatal("Get after Delete: expected error, got nil")
	}
	if !errors.Is(err, ErrNotExist) {
		t.Fatalf("Get after Delete: err = %v, want ErrNotExist", err)
	}
}

func TestIntegrationGCSDeleteMissing(t *testing.T) {
	bucket := os.Getenv("STORAGE_TEST_BUCKET")
	if bucket == "" {
		t.Skip("STORAGE_TEST_BUCKET not set")
	}
	ctx := context.Background()
	uri := "gs://" + bucket + "/" + newKeyPrefix(t) + "/nope.txt"

	err := Delete(ctx, uri)
	if err == nil {
		t.Fatal("expected ErrNotExist, got nil")
	}
	if !errors.Is(err, ErrNotExist) {
		t.Fatalf("Delete(missing): err = %v, want ErrNotExist", err)
	}
}
