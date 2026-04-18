package storage

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMemStoreRoundTrip(t *testing.T) {
	m := newMemStore()
	ctx := context.Background()
	uri := "mem://" + t.Name() + "/hello.txt"

	if err := m.PutBytes(ctx, uri, []byte("hi"), "text/plain"); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	got, err := m.Get(ctx, uri)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "hi" {
		t.Fatalf("Get = %q, want %q", got, "hi")
	}

	objs, err := m.List(ctx, "mem://"+t.Name())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("List len = %d, want 1", len(objs))
	}
	if objs[0].ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want text/plain", objs[0].ContentType)
	}
	if objs[0].Size != 2 {
		t.Errorf("Size = %d, want 2", objs[0].Size)
	}

	if err := m.Delete(ctx, uri); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := m.Get(ctx, uri); !errors.Is(err, ErrNotExist) {
		t.Fatalf("Get after Delete: err = %v, want ErrNotExist", err)
	}
}

func TestMemStoreGetBytesAreCopied(t *testing.T) {
	m := newMemStore()
	ctx := context.Background()
	uri := "mem://" + t.Name() + "/k"
	payload := []byte("original")

	if err := m.PutBytes(ctx, uri, payload, ""); err != nil {
		t.Fatal(err)
	}
	payload[0] = 'X' // mutate caller-owned buffer

	got, err := m.Get(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("Get = %q, want %q (store must copy bytes on Put)", got, "original")
	}
	got[0] = 'Y' // mutate returned buffer

	got2, _ := m.Get(ctx, uri)
	if string(got2) != "original" {
		t.Fatalf("second Get = %q, want %q (store must copy bytes on Get)", got2, "original")
	}
}

func TestMemStoreDeleteMissing(t *testing.T) {
	m := newMemStore()
	err := m.Delete(context.Background(), "mem://"+t.Name()+"/missing")
	if !errors.Is(err, ErrNotExist) {
		t.Fatalf("Delete(missing): err = %v, want ErrNotExist", err)
	}
}

func TestMemStoreListPrefix(t *testing.T) {
	m := newMemStore()
	ctx := context.Background()
	ns := "mem://" + t.Name()
	for _, k := range []string{"/a/1", "/a/2", "/b/3"} {
		if err := m.PutBytes(ctx, ns+k, []byte("x"), ""); err != nil {
			t.Fatal(err)
		}
	}
	objs, err := m.List(ctx, ns+"/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 {
		t.Fatalf("List len = %d, want 2: %+v", len(objs), objs)
	}
	for _, o := range objs {
		if !strings.Contains(o.URI, "/a/") {
			t.Errorf("unexpected URI in /a prefix: %q", o.URI)
		}
	}
}

func TestMemKeyRejectsEmpty(t *testing.T) {
	_, err := memKey("mem://")
	if !errors.Is(err, ErrInvalidURI) {
		t.Fatalf("memKey(\"mem://\") err = %v, want ErrInvalidURI", err)
	}
}

func TestMemStoreIsolatedByNamespace(t *testing.T) {
	m := newMemStore()
	ctx := context.Background()
	if err := m.PutBytes(ctx, "mem://ns1/key", []byte("one"), ""); err != nil {
		t.Fatal(err)
	}
	if err := m.PutBytes(ctx, "mem://ns2/key", []byte("two"), ""); err != nil {
		t.Fatal(err)
	}
	a, _ := m.Get(ctx, "mem://ns1/key")
	b, _ := m.Get(ctx, "mem://ns2/key")
	if bytes.Equal(a, b) {
		t.Fatalf("namespaces leaked: ns1=%q ns2=%q", a, b)
	}
}
