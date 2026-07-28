package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const sidecarSuffix = ".meta.json"

// fileStore serializes writes with mu so generation reads and bumps are
// atomic within one process. The generation itself persists in the
// <path>.meta.json sidecar, but nothing locks the files themselves, so
// conditional writes only guard against racing goroutines, not against a
// second process — consistent with the backend's local-development role.
type fileStore struct {
	mu sync.Mutex
}

type sidecar struct {
	ContentType string `json:"content_type,omitempty"`
	Generation  int64  `json:"generation,omitempty"`
}

func pathFromURI(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("storage: parse %q: %w", uri, err)
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("storage: expected file scheme, got %q", u.Scheme)
	}
	if u.Host != "" {
		return "", fmt.Errorf("%w: non-empty host %q in %q", ErrInvalidURI, u.Host, uri)
	}
	if u.Path == "" {
		return "", fmt.Errorf("%w: empty path in %q", ErrInvalidURI, uri)
	}
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == ".." || seg == "." {
			return "", fmt.Errorf("%w: %q", ErrInvalidURI, uri)
		}
	}
	p := filepath.Clean(filepath.FromSlash(u.Path))
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%w: not absolute: %q", ErrInvalidURI, uri)
	}
	return p, nil
}

func (f *fileStore) Get(ctx context.Context, uri string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, err := pathFromURI(uri)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p) // #nosec G304 -- path validated by pathFromURI
	if err != nil {
		return nil, fmt.Errorf("storage: get %q: %w", uri, err)
	}
	return data, nil
}

func (f *fileStore) GetWithGeneration(ctx context.Context, uri string) ([]byte, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	p, err := pathFromURI(uri)
	if err != nil {
		return nil, 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	data, err := os.ReadFile(p) // #nosec G304 -- path validated by pathFromURI
	if err != nil {
		return nil, 0, fmt.Errorf("storage: get %q: %w", uri, err)
	}
	return data, generationFor(p), nil
}

func (f *fileStore) PutFile(ctx context.Context, uri, source string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := pathFromURI(uri)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("storage: mkdir for %q: %w", uri, err)
	}
	src, err := os.Open(source) // #nosec G304 -- source is a caller-supplied local path
	if err != nil {
		return fmt.Errorf("storage: open source %q: %w", source, err)
	}
	defer func() { _ = src.Close() }()
	f.mu.Lock()
	defer f.mu.Unlock()
	gen := f.generationLocked(p)
	dst, err := os.Create(p) // #nosec G304 -- path validated by pathFromURI
	if err != nil {
		return fmt.Errorf("storage: create %q: %w", uri, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("storage: write %q: %w", uri, err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("storage: close %q: %w", uri, err)
	}
	if err := f.writeSidecarLocked(p, "", gen+1); err != nil {
		return fmt.Errorf("storage: write sidecar for %q: %w", uri, err)
	}
	return nil
}

func (f *fileStore) PutBytes(ctx context.Context, uri string, data []byte, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := pathFromURI(uri)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("storage: mkdir for %q: %w", uri, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	gen := f.generationLocked(p)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("storage: write %q: %w", uri, err)
	}
	if err := f.writeSidecarLocked(p, contentType, gen+1); err != nil {
		return fmt.Errorf("storage: write sidecar for %q: %w", uri, err)
	}
	return nil
}

func (f *fileStore) PutBytesIfGeneration(ctx context.Context, uri string, data []byte, contentType string, ifGeneration int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := pathFromURI(uri)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	// A missing object has generation 0, so one comparison covers both the
	// "must not exist yet" create and the "must still match" replace.
	if f.generationLocked(p) != ifGeneration {
		return fmt.Errorf("storage: put %q: %w", uri, ErrPreconditionFailed)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("storage: mkdir for %q: %w", uri, err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("storage: write %q: %w", uri, err)
	}
	if err := f.writeSidecarLocked(p, contentType, ifGeneration+1); err != nil {
		return fmt.Errorf("storage: write sidecar for %q: %w", uri, err)
	}
	return nil
}

func (f *fileStore) Delete(ctx context.Context, uri string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := pathFromURI(uri)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.Remove(p); err != nil {
		return fmt.Errorf("storage: delete %q: %w", uri, err)
	}
	if err := os.Remove(p + sidecarSuffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storage: delete sidecar for %q: %w", uri, err)
	}
	return nil
}

func (f *fileStore) List(ctx context.Context, uri string) ([]Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := pathFromURI(uri)
	if err != nil {
		return nil, err
	}
	var objs []Object
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		if strings.HasSuffix(p, sidecarSuffix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sc := readSidecar(p)
		objs = append(objs, Object{
			URI:         fileURIFor(p),
			Size:        info.Size(),
			ContentType: sc.ContentType,
			Updated:     info.ModTime(),
			Generation:  seededGeneration(sc.Generation),
		})
		return nil
	})
	if errors.Is(walkErr, fs.ErrNotExist) {
		return nil, nil
	}
	if walkErr != nil {
		return nil, fmt.Errorf("storage: list %q: %w", uri, walkErr)
	}
	// Guarantee lexicographic order across backends. WalkDir already visits
	// in lex order, but the sort makes the contract explicit and survives
	// future WalkDir implementation changes.
	sort.Slice(objs, func(i, j int) bool { return objs[i].URI < objs[j].URI })
	return objs, nil
}

// fileURIFor constructs a file:// URI for an absolute filesystem path,
// percent-encoding any characters that would otherwise be ambiguous.
func fileURIFor(p string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(p)}
	return u.String()
}

// generationLocked returns the current generation of the object at path:
// the sidecar's recorded value, 1 for a pre-existing file with none, and 0
// when the file does not exist. Callers must hold f.mu.
func (f *fileStore) generationLocked(path string) int64 {
	if _, err := os.Stat(path); err != nil {
		return 0
	}
	return generationFor(path)
}

// generationFor returns the generation of an existing file: its sidecar's
// recorded value, or 1 when the file predates generation tracking.
func generationFor(path string) int64 {
	return seededGeneration(readSidecar(path).Generation)
}

// seededGeneration maps an unrecorded generation to 1 so every existing
// object reports a non-zero generation — 0 stays reserved for "the object
// must not exist yet" in PutBytesIfGeneration.
func seededGeneration(gen int64) int64 {
	if gen == 0 {
		return 1
	}
	return gen
}

// writeSidecarLocked records generation and contentType in the sidecar. An
// empty contentType preserves the previously recorded one. Callers must
// hold f.mu.
func (f *fileStore) writeSidecarLocked(path, contentType string, generation int64) error {
	sc := readSidecar(path)
	sc.Generation = generation
	if contentType != "" {
		sc.ContentType = contentType
	}
	meta, err := json.Marshal(sc)
	if err != nil {
		return err
	}
	return os.WriteFile(path+sidecarSuffix, meta, 0o600)
}

func readSidecar(path string) sidecar {
	data, err := os.ReadFile(path + sidecarSuffix) // #nosec G304 -- path comes from a validated URI or filepath.WalkDir rooted at a validated prefix
	if err != nil {
		return sidecar{}
	}
	var s sidecar
	if err := json.Unmarshal(data, &s); err != nil {
		return sidecar{}
	}
	return s
}

var _ ConditionalStore = (*fileStore)(nil)
