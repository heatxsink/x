package storage

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fileURI(t *testing.T, root, key string) string {
	t.Helper()
	return "file://" + filepath.ToSlash(filepath.Join(root, key))
}

func TestFileStoreRoundTrip(t *testing.T) {
	var fs fileStore
	ctx := context.Background()
	dir := t.TempDir()
	uri := fileURI(t, dir, "nested/dir/hello.txt")

	if err := fs.PutBytes(ctx, uri, []byte("hi"), "text/plain"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	got, err := fs.Get(ctx, uri)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "hi" {
		t.Fatalf("Get = %q, want %q", got, "hi")
	}
}

func TestFileStoreSidecar(t *testing.T) {
	var fs fileStore
	ctx := context.Background()
	dir := t.TempDir()
	uri := fileURI(t, dir, "doc.json")

	if err := fs.PutBytes(ctx, uri, []byte("{}"), "application/json"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}

	objs, err := fs.List(ctx, "file://"+filepath.ToSlash(dir))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("List len = %d, want 1 (sidecar must be hidden): %+v", len(objs), objs)
	}
	if objs[0].ContentType != "application/json" {
		t.Fatalf("ContentType = %q, want application/json", objs[0].ContentType)
	}

	if err := fs.Delete(ctx, uri); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	sidecar := filepath.Join(dir, "doc.json"+sidecarSuffix)
	if _, err := os.Stat(sidecar); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sidecar still present after Delete: err=%v", err)
	}
}

func TestFileStorePutFile(t *testing.T) {
	var fs fileStore
	ctx := context.Background()
	dir := t.TempDir()
	source := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := fileURI(t, dir, "out/dest.bin")
	if err := fs.PutFile(ctx, dst, source); err != nil {
		t.Fatalf("PutFile: %v", err)
	}
	got, err := fs.Get(ctx, dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Fatalf("Get = %q, want %q", got, "payload")
	}
}

func TestFileStoreListOrderAndRecursion(t *testing.T) {
	var fs fileStore
	ctx := context.Background()
	dir := t.TempDir()
	keys := []string{"z/last.txt", "a/first.txt", "m/mid.txt"}
	for _, k := range keys {
		if err := fs.PutBytes(ctx, fileURI(t, dir, k), []byte(k), ""); err != nil {
			t.Fatal(err)
		}
	}
	objs, err := fs.List(ctx, "file://"+filepath.ToSlash(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 3 {
		t.Fatalf("got %d objects, want 3", len(objs))
	}
	for i, want := range []string{"a/first.txt", "m/mid.txt", "z/last.txt"} {
		if !strings.HasSuffix(objs[i].URI, want) {
			t.Fatalf("objs[%d].URI = %q, want suffix %q", i, objs[i].URI, want)
		}
	}
}

func TestFileStoreListMissingRoot(t *testing.T) {
	var fs fileStore
	ctx := context.Background()
	objs, err := fs.List(ctx, "file:///definitely/not/a/real/path-"+t.Name())
	if err != nil {
		t.Fatalf("List on missing root: %v", err)
	}
	if objs != nil {
		t.Fatalf("got %d objects, want nil", len(objs))
	}
}

func TestFileStoreGetMissing(t *testing.T) {
	var fs fileStore
	ctx := context.Background()
	_, err := fs.Get(ctx, "file:///no/such/file-"+t.Name())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	cases := []string{
		"file:///var/data/../etc/passwd",
		"file:///../etc/passwd",
		"file://relative/path",
		"file://",
	}
	for _, uri := range cases {
		t.Run(uri, func(t *testing.T) {
			_, err := pathFromURI(uri)
			if err == nil {
				t.Fatalf("pathFromURI(%q) = nil, want error", uri)
			}
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("pathFromURI(%q) err = %v, want ErrInvalidPath", uri, err)
			}
		})
	}
}

func TestFileStoreDeleteNoSidecar(t *testing.T) {
	var fsstore fileStore
	ctx := context.Background()
	dir := t.TempDir()
	uri := fileURI(t, dir, "plain.bin")
	if err := fsstore.PutBytes(ctx, uri, []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	if err := fsstore.Delete(ctx, uri); err != nil {
		t.Fatalf("Delete without sidecar: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plain.bin")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file still exists: err=%v", err)
	}
}
