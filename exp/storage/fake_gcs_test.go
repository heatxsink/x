package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
)

func newFakeGCSStore(t *testing.T, bucket string) *gcsStore {
	t.Helper()
	server, err := fakestorage.NewServerWithOptions(fakestorage.Options{
		NoListener: true,
	})
	if err != nil {
		t.Fatalf("fake gcs server: %v", err)
	}
	t.Cleanup(server.Stop)
	server.CreateBucketWithOpts(fakestorage.CreateBucketOpts{Name: bucket})

	s := &gcsStore{client: server.Client()}
	// Consume the sync.Once so getClient returns the injected client.
	s.once.Do(func() {})
	return s
}

func TestGCSRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newFakeGCSStore(t, "test-bucket")
	uri := "gs://test-bucket/hello.txt"
	payload := []byte("hello fake gcs")

	if err := s.PutBytes(ctx, uri, payload, "text/plain"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	got, err := s.Get(ctx, uri)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("Get = %q, want %q", got, payload)
	}

	objs, err := s.List(ctx, "gs://test-bucket/")
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
		}
	}
	if !found {
		t.Fatalf("List missing key: %+v", objs)
	}

	if err := s.Delete(ctx, uri); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, uri); !errors.Is(err, ErrNotExist) {
		t.Fatalf("Get after Delete: err = %v, want ErrNotExist", err)
	}
}

func TestGCSGetMissing(t *testing.T) {
	ctx := context.Background()
	s := newFakeGCSStore(t, "test-bucket")
	_, err := s.Get(ctx, "gs://test-bucket/missing.txt")
	if !errors.Is(err, ErrNotExist) {
		t.Fatalf("Get missing: err = %v, want ErrNotExist", err)
	}
}

func TestGCSDeleteMissing(t *testing.T) {
	ctx := context.Background()
	s := newFakeGCSStore(t, "test-bucket")
	err := s.Delete(ctx, "gs://test-bucket/missing.txt")
	if !errors.Is(err, ErrNotExist) {
		t.Fatalf("Delete missing: err = %v, want ErrNotExist", err)
	}
}

func TestGCSPutFile(t *testing.T) {
	ctx := context.Background()
	s := newFakeGCSStore(t, "test-bucket")
	payload := []byte("from file")
	src := filepath.Join(t.TempDir(), "src.txt")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	uri := "gs://test-bucket/from-file.txt"
	if err := s.PutFile(ctx, uri, src); err != nil {
		t.Fatalf("PutFile: %v", err)
	}
	got, err := s.Get(ctx, uri)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("Get = %q, want %q", got, payload)
	}
}

func TestGCSListPrefixScoped(t *testing.T) {
	ctx := context.Background()
	s := newFakeGCSStore(t, "test-bucket")
	for _, k := range []string{"a/1.txt", "a/2.txt", "b/1.txt"} {
		if err := s.PutBytes(ctx, "gs://test-bucket/"+k, []byte(k), "text/plain"); err != nil {
			t.Fatalf("PutBytes %q: %v", k, err)
		}
	}
	objs, err := s.List(ctx, "gs://test-bucket/a/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("List under a/ = %d objects, want 2: %+v", len(objs), objs)
	}
	for _, o := range objs {
		if !strings.Contains(o.URI, "/a/") {
			t.Errorf("unexpected URI outside a/: %q", o.URI)
		}
	}
}
