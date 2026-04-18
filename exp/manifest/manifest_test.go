package manifest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/heatxsink/x/exp/storage"
)

func newTestManifest(t *testing.T) *Manifest {
	t.Helper()
	return New("mem://"+t.Name(), "2024-01-01")
}

func TestSaveLoadRoundTrip(t *testing.T) {
	m := newTestManifest(t)
	ctx := context.Background()

	want := []*Item{
		{Published: time.Now().UTC().Truncate(time.Second), Version: Version{1, 2, 3}, Prefix: "ABC"},
		{Published: time.Now().UTC().Truncate(time.Second), Version: Version{1, 2, 4}, Prefix: "DEF"},
	}
	if err := m.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := m.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Prefix != want[i].Prefix {
			t.Errorf("Item[%d].Prefix = %q, want %q", i, got[i].Prefix, want[i].Prefix)
		}
		if got[i].Version != want[i].Version {
			t.Errorf("Item[%d].Version = %v, want %v", i, got[i].Version, want[i].Version)
		}
	}
}

func TestLoadMissingManifest(t *testing.T) {
	m := newTestManifest(t)
	_, err := m.Load(context.Background())
	if !errors.Is(err, storage.ErrNotExist) {
		t.Fatalf("Load on empty baseURI: err = %v, want ErrNotExist", err)
	}
}

func TestClean(t *testing.T) {
	baseURI := "mem://" + t.Name()
	m := New(baseURI, "2024-01-01")
	ctx := context.Background()

	seed := []string{
		"/KEEP/a.html",
		"/KEEP/b.html",
		"/DROP/c.html",
		"/DROP/d.html",
		"/manifest.json",
	}
	for _, k := range seed {
		if err := storage.PutBytes(ctx, baseURI+k, []byte("x"), ""); err != nil {
			t.Fatalf("seed PutBytes %q: %v", k, err)
		}
	}

	items := []*Item{{Prefix: "KEEP"}}
	if err := m.Clean(ctx, items, []string{"manifest.json"}); err != nil {
		t.Fatalf("Clean: %v", err)
	}

	objs, err := storage.List(ctx, baseURI)
	if err != nil {
		t.Fatal(err)
	}
	remaining := map[string]bool{}
	for _, o := range objs {
		remaining[o.URI] = true
	}
	wantRemain := []string{baseURI + "/KEEP/a.html", baseURI + "/KEEP/b.html", baseURI + "/manifest.json"}
	for _, w := range wantRemain {
		if !remaining[w] {
			t.Errorf("expected %q to remain, gone", w)
		}
	}
	for _, d := range []string{baseURI + "/DROP/c.html", baseURI + "/DROP/d.html"} {
		if remaining[d] {
			t.Errorf("expected %q to be deleted, still present", d)
		}
	}
}

func TestVersionString(t *testing.T) {
	v := Version{Major: 1, Minor: 23, Point: 4}
	if got := v.String(); got != "1.23.4" {
		t.Fatalf("Version.String = %q, want %q", got, "1.23.4")
	}
}

func TestCreateHashUnique(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		h := createHash()
		if _, dup := seen[h]; dup {
			t.Fatalf("createHash produced duplicate after %d calls: %q", i, h)
		}
		seen[h] = struct{}{}
	}
}
