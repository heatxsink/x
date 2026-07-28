package storage

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/api/googleapi"
)

// condBackend pairs a ConditionalStore with a URI factory so the same
// conditional-write contract can be asserted against every built-in backend.
type condBackend struct {
	name  string
	store ConditionalStore
	uri   func(key string) string
}

func condBackends(t *testing.T) []condBackend {
	t.Helper()
	dir := t.TempDir()
	return []condBackend{
		{
			name:  "gcs",
			store: newFakeGCSStore(t, "cond-bucket"),
			uri:   func(key string) string { return "gs://cond-bucket/" + key },
		},
		{
			name:  "file",
			store: &fileStore{},
			uri:   func(key string) string { return "file://" + filepath.ToSlash(filepath.Join(dir, key)) },
		},
		{
			name:  "mem",
			store: newMemStore(),
			uri:   func(key string) string { return "mem://" + t.Name() + "/" + key },
		},
	}
}

func TestConditionalLifecycle(t *testing.T) {
	ctx := context.Background()
	for _, b := range condBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			uri := b.uri("lifecycle.json")

			// Missing object wraps ErrNotExist, same as Get.
			if _, _, err := b.store.GetWithGeneration(ctx, uri); !errors.Is(err, ErrNotExist) {
				t.Fatalf("GetWithGeneration missing: err = %v, want ErrNotExist", err)
			}

			// ifGeneration 0 creates when the object does not exist yet.
			if err := b.store.PutBytesIfGeneration(ctx, uri, []byte(`{"v":1}`), "application/json", 0); err != nil {
				t.Fatalf("PutBytesIfGeneration create: %v", err)
			}
			// ...and conflicts once it does.
			if err := b.store.PutBytesIfGeneration(ctx, uri, []byte(`clobber`), "application/json", 0); !errors.Is(err, ErrPreconditionFailed) {
				t.Fatalf("create-over-existing: err = %v, want ErrPreconditionFailed", err)
			}

			data, gen, err := b.store.GetWithGeneration(ctx, uri)
			if err != nil {
				t.Fatalf("GetWithGeneration: %v", err)
			}
			if string(data) != `{"v":1}` || gen == 0 {
				t.Fatalf("GetWithGeneration = (%q, %d), want created content with a non-zero generation", data, gen)
			}

			// A write with the matching generation succeeds and changes it.
			if err := b.store.PutBytesIfGeneration(ctx, uri, []byte(`{"v":2}`), "application/json", gen); err != nil {
				t.Fatalf("PutBytesIfGeneration matched: %v", err)
			}
			data, gen2, err := b.store.GetWithGeneration(ctx, uri)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != `{"v":2}` {
				t.Fatalf("content after matched write = %q, want %q", data, `{"v":2}`)
			}
			if gen2 == gen {
				t.Fatal("generation did not change after a successful conditional write")
			}

			// The stale generation now conflicts and must not clobber.
			if err := b.store.PutBytesIfGeneration(ctx, uri, []byte(`clobber`), "application/json", gen); !errors.Is(err, ErrPreconditionFailed) {
				t.Fatalf("stale-generation write: err = %v, want ErrPreconditionFailed", err)
			}
			data, gen3, err := b.store.GetWithGeneration(ctx, uri)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != `{"v":2}` || gen3 != gen2 {
				t.Fatalf("after rejected write: (%q, %d), want untouched (%q, %d)", data, gen3, `{"v":2}`, gen2)
			}
		})
	}
}

func TestConditionalUnconditionalPutInvalidates(t *testing.T) {
	ctx := context.Background()
	for _, b := range condBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			uri := b.uri("invalidate.txt")
			if err := b.store.PutBytesIfGeneration(ctx, uri, []byte("first"), "text/plain", 0); err != nil {
				t.Fatalf("create: %v", err)
			}
			_, gen, err := b.store.GetWithGeneration(ctx, uri)
			if err != nil {
				t.Fatal(err)
			}

			// An unconditional write must bump the generation so the
			// outstanding conditional writer loses.
			if err := b.store.PutBytes(ctx, uri, []byte("unconditional"), "text/plain"); err != nil {
				t.Fatalf("PutBytes: %v", err)
			}
			if err := b.store.PutBytesIfGeneration(ctx, uri, []byte("stale"), "text/plain", gen); !errors.Is(err, ErrPreconditionFailed) {
				t.Fatalf("conditional write after PutBytes: err = %v, want ErrPreconditionFailed", err)
			}
			got, err := b.store.Get(ctx, uri)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, []byte("unconditional")) {
				t.Fatalf("content = %q, want %q", got, "unconditional")
			}
		})
	}
}

func TestConditionalRetryLoop(t *testing.T) {
	ctx := context.Background()
	for _, b := range condBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			uri := b.uri("retry.txt")
			if err := b.store.PutBytesIfGeneration(ctx, uri, []byte("base"), "text/plain", 0); err != nil {
				t.Fatalf("create: %v", err)
			}

			// Classic read-modify-write loop with one simulated conflict: a
			// rival writer sneaks in after the first read.
			conflicted := false
			for attempt := 0; ; attempt++ {
				if attempt > 2 {
					t.Fatal("retry loop did not converge")
				}
				data, gen, err := b.store.GetWithGeneration(ctx, uri)
				if err != nil {
					t.Fatal(err)
				}
				if !conflicted {
					conflicted = true
					if err := b.store.PutBytesIfGeneration(ctx, uri, []byte(string(data)+" rival"), "text/plain", gen); err != nil {
						t.Fatalf("rival write: %v", err)
					}
				}
				err = b.store.PutBytesIfGeneration(ctx, uri, []byte(string(data)+" mine"), "text/plain", gen)
				if errors.Is(err, ErrPreconditionFailed) {
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				break
			}

			got, err := b.store.Get(ctx, uri)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "base rival mine" {
				t.Fatalf("content = %q, want %q (retry must rebase on the rival's write)", got, "base rival mine")
			}
		})
	}
}

func TestConditionalListGenerationConsistent(t *testing.T) {
	ctx := context.Background()
	for _, b := range condBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			uri := b.uri("listgen/obj.txt")
			if err := b.store.PutBytesIfGeneration(ctx, uri, []byte("x"), "text/plain", 0); err != nil {
				t.Fatalf("create: %v", err)
			}
			_, gen, err := b.store.GetWithGeneration(ctx, uri)
			if err != nil {
				t.Fatal(err)
			}
			objs, err := b.store.List(ctx, b.uri("listgen/"))
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(objs) != 1 {
				t.Fatalf("List len = %d, want 1: %+v", len(objs), objs)
			}
			if objs[0].Generation != gen {
				t.Fatalf("List Generation = %d, want %d (must match GetWithGeneration)", objs[0].Generation, gen)
			}
		})
	}
}

// TestFileStorePreexistingObjectGeneration pins the seeding rule: a file
// written outside the store (no sidecar) reports generation 1, so a stale
// gen-0 create cannot clobber it but a matched write can replace it.
func TestFileStorePreexistingObjectGeneration(t *testing.T) {
	fs := &fileStore{}
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "legacy.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := "file://" + filepath.ToSlash(filepath.Join(dir, "legacy.txt"))

	data, gen, err := fs.GetWithGeneration(ctx, uri)
	if err != nil {
		t.Fatalf("GetWithGeneration: %v", err)
	}
	if string(data) != "old" || gen != 1 {
		t.Fatalf("GetWithGeneration = (%q, %d), want (%q, 1)", data, gen, "old")
	}
	if err := fs.PutBytesIfGeneration(ctx, uri, []byte("clobber"), "", 0); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("gen-0 create over pre-existing file: err = %v, want ErrPreconditionFailed", err)
	}
	if err := fs.PutBytesIfGeneration(ctx, uri, []byte("new"), "", gen); err != nil {
		t.Fatalf("matched write over pre-existing file: %v", err)
	}
	_, gen2, err := fs.GetWithGeneration(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	if gen2 != gen+1 {
		t.Fatalf("generation after write = %d, want %d", gen2, gen+1)
	}
}

// TestGCSPreconditionMapping pins the 412 -> ErrPreconditionFailed
// translation independent of the fake server's behavior.
func TestGCSPreconditionMapping(t *testing.T) {
	if !isPreconditionFailed(&googleapi.Error{Code: http.StatusPreconditionFailed}) {
		t.Error("412 googleapi error not recognized as a precondition failure")
	}
	if isPreconditionFailed(&googleapi.Error{Code: http.StatusTooManyRequests}) {
		t.Error("429 misclassified as a precondition failure")
	}
	if isPreconditionFailed(errors.New("plain")) {
		t.Error("non-API error misclassified as a precondition failure")
	}
}

func TestPackageConditionalHelpers(t *testing.T) {
	ctx := context.Background()
	uri := "mem://" + t.Name() + "/helper.txt"

	if err := PutBytesIfGeneration(ctx, uri, []byte("v1"), "text/plain", 0); err != nil {
		t.Fatalf("PutBytesIfGeneration: %v", err)
	}
	data, gen, err := GetWithGeneration(ctx, uri)
	if err != nil {
		t.Fatalf("GetWithGeneration: %v", err)
	}
	if string(data) != "v1" || gen == 0 {
		t.Fatalf("GetWithGeneration = (%q, %d), want (%q, non-zero)", data, gen, "v1")
	}
	if err := PutBytesIfGeneration(ctx, uri, []byte("v2"), "text/plain", gen); err != nil {
		t.Fatalf("PutBytesIfGeneration matched: %v", err)
	}
}

// plainStore implements Store but not ConditionalStore, standing in for an
// external backend without conditional support.
type plainStore struct{ Store }

func TestPackageConditionalHelpersUnsupported(t *testing.T) {
	t.Cleanup(resetForTest)
	storesMu.Lock()
	stores["plain"] = plainStore{}
	storesMu.Unlock()

	ctx := context.Background()
	if _, _, err := GetWithGeneration(ctx, "plain://ns/key"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("GetWithGeneration on plain backend: err = %v, want ErrUnsupported", err)
	}
	if err := PutBytesIfGeneration(ctx, "plain://ns/key", nil, "", 0); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("PutBytesIfGeneration on plain backend: err = %v, want ErrUnsupported", err)
	}
}
