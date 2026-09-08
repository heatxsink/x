package gcs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/fsouza/fake-gcs-server/fakestorage"
)

// freePort reserves an ephemeral port and releases it, so the fake server can
// be constructed with a known ExternalURL before it starts listening.
func freePort(t *testing.T) uint16 {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is %T, want *net.TCPAddr", l.Addr())
	}
	port := addr.Port
	if err := l.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return uint16(port) // #nosec G115 -- an ephemeral port always fits in uint16
}

// newFakeGCS starts an in-process fake GCS server and points the production
// storage client at it via STORAGE_EMULATOR_HOST. The functions in this
// package construct their own storage.Client with no injection point, so the
// emulator env var is the only seam available.
//
// ExternalURL must be set explicitly: without it the fake server hands back a
// host-relative MediaLink and every object read fails to resolve.
func newFakeGCS(t *testing.T, bucket string) *fakestorage.Server {
	t.Helper()
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	server, err := fakestorage.NewServerWithOptions(fakestorage.Options{
		Scheme:      "http",
		Host:        "127.0.0.1",
		Port:        port,
		ExternalURL: "http://" + addr,
		PublicHost:  addr,
	})
	if err != nil {
		t.Fatalf("fake gcs server: %v", err)
	}
	t.Cleanup(server.Stop)
	server.CreateBucketWithOpts(fakestorage.CreateBucketOpts{Name: bucket})

	// storage.NewClient reads this and skips authentication entirely.
	t.Setenv("STORAGE_EMULATOR_HOST", addr)
	return server
}

func TestPutBytesGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")
	payload := []byte("hello deprecated gcs")

	if err := PutBytes(ctx, "test-bucket", "hello.txt", payload, "text/plain"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	got, err := Get(ctx, "test-bucket", "hello.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("Get = %q, want %q", got, payload)
	}
}

// PutBytes advertises a content type and a no-cache policy; both are part of
// the contract callers depend on for browser-served objects.
func TestPutBytesSetsObjectMetadata(t *testing.T) {
	ctx := context.Background()
	server := newFakeGCS(t, "test-bucket")

	if err := PutBytes(ctx, "test-bucket", "styled.css", []byte("body{}"), "text/css"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	attrs, err := server.GetObject("test-bucket", "styled.css")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if attrs.ContentType != "text/css" {
		t.Errorf("ContentType = %q, want text/css", attrs.ContentType)
	}
	if want := "private, max-age=0, no-transform"; attrs.CacheControl != want {
		t.Errorf("CacheControl = %q, want %q", attrs.CacheControl, want)
	}
}

func TestPutBytesEmptyPayload(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")

	if err := PutBytes(ctx, "test-bucket", "empty.txt", nil, "text/plain"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	got, err := Get(ctx, "test-bucket", "empty.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Get = %q, want empty", got)
	}
}

func TestGetMissingObject(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")

	_, err := Get(ctx, "test-bucket", "nope.txt")
	if !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("Get missing: err = %v, want storage.ErrObjectNotExist", err)
	}
}

func TestGetMissingBucket(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")

	if _, err := Get(ctx, "no-such-bucket", "nope.txt"); err == nil {
		t.Fatal("Get from missing bucket: err = nil, want error")
	}
}

func TestPutFileRoundTrip(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")
	payload := []byte("contents from local disk")
	src := filepath.Join(t.TempDir(), "src.txt")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := PutFile(ctx, "test-bucket", "uploaded.txt", src); err != nil {
		t.Fatalf("PutFile: %v", err)
	}
	got, err := Get(ctx, "test-bucket", "uploaded.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("Get = %q, want %q", got, payload)
	}
}

func TestPutFileMissingSource(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")

	err := PutFile(ctx, "test-bucket", "uploaded.txt", filepath.Join(t.TempDir(), "absent.txt"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("PutFile missing source: err = %v, want os.ErrNotExist", err)
	}
}

func TestDeleteRemovesObject(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")
	if err := PutBytes(ctx, "test-bucket", "doomed.txt", []byte("x"), "text/plain"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}

	if err := Delete(ctx, "test-bucket", "doomed.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := Get(ctx, "test-bucket", "doomed.txt"); !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("Get after Delete: err = %v, want storage.ErrObjectNotExist", err)
	}
}

func TestDeleteMissingObject(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")

	err := Delete(ctx, "test-bucket", "never-existed.txt")
	if !errors.Is(err, storage.ErrObjectNotExist) {
		t.Fatalf("Delete missing: err = %v, want storage.ErrObjectNotExist", err)
	}
}

func TestListReturnsAllObjects(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")
	keys := []string{"a/1.txt", "a/2.txt", "b/1.txt"}
	for _, k := range keys {
		if err := PutBytes(ctx, "test-bucket", k, []byte(k), "text/plain"); err != nil {
			t.Fatalf("PutBytes %q: %v", k, err)
		}
	}

	objects, err := List(ctx, "test-bucket")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := make([]string, 0, len(objects))
	for _, o := range objects {
		got = append(got, o.Name)
	}
	sort.Strings(got)
	if len(got) != len(keys) {
		t.Fatalf("List = %v, want %v", got, keys)
	}
	for i := range keys {
		if got[i] != keys[i] {
			t.Fatalf("List = %v, want %v", got, keys)
		}
	}
}

// List drains the iterator; an empty bucket must yield no objects and no error
// rather than surfacing iterator.Done to the caller.
func TestListEmptyBucket(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "empty-bucket")

	objects, err := List(ctx, "empty-bucket")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objects) != 0 {
		t.Fatalf("List = %d objects, want 0", len(objects))
	}
}

func TestListMissingBucket(t *testing.T) {
	ctx := context.Background()
	newFakeGCS(t, "test-bucket")

	if _, err := List(ctx, "no-such-bucket"); err == nil {
		t.Fatal("List on missing bucket: err = nil, want error")
	}
}

// A canceled context must not be silently ignored by the client construction
// or the object read path.
func TestGetHonorsCanceledContext(t *testing.T) {
	newFakeGCS(t, "test-bucket")
	if err := PutBytes(context.Background(), "test-bucket", "x.txt", []byte("x"), "text/plain"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Get(ctx, "test-bucket", "x.txt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get with canceled ctx: err = %v, want context.Canceled", err)
	}
}
