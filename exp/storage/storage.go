// Package storage provides a URI-addressable blob store that dispatches
// between Google Cloud Storage (gs://bucket/key) and the local filesystem
// (file:///abs/path). Callers switch backends by changing a URI; the
// surface never exposes backend-specific types.
//
// The file:// backend currently targets POSIX paths. Windows file URIs of
// the form file:///C:/path are not handled in this version; support may be
// added later.
//
// For(uri) memoizes one Store per scheme, so the GCS client — and its
// underlying HTTP/gRPC connection pool — is reused across calls within a
// process.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sync"
	"time"
)

// Object describes a stored blob as returned by List.
type Object struct {
	URI         string
	Size        int64
	ContentType string
	Updated     time.Time
}

// Store is the URI-addressable blob interface. Both backends accept the
// same URI forms they recognize by scheme.
type Store interface {
	// Get reads the entire object at uri into memory. Returns an error
	// wrapping ErrNotExist when the object does not exist.
	Get(ctx context.Context, uri string) ([]byte, error)

	// PutFile uploads the contents of source to uri. The source parameter
	// is a local filesystem path; callers are responsible for validating
	// it when it originates from untrusted input.
	PutFile(ctx context.Context, uri, source string) error

	// PutBytes writes data to uri. On gs:// URIs, contentType becomes the
	// object's Content-Type metadata. On file:// URIs, a <path>.meta.json
	// sidecar records it; an empty contentType skips the sidecar write.
	PutBytes(ctx context.Context, uri string, data []byte, contentType string) error

	// Delete removes the object at uri. On file:// URIs, an adjacent
	// <path>.meta.json sidecar is also removed when present. Returns an
	// error wrapping ErrNotExist when the object does not exist.
	Delete(ctx context.Context, uri string) error

	// List returns all objects under uri, recursively, in lexicographic
	// order by URI. On gs:// URIs, uri is treated as a prefix: every
	// object whose name begins with the URI's key portion is returned.
	// Include a trailing "/" to scope to a directory-like prefix. On
	// file:// URIs, uri is a directory root.
	List(ctx context.Context, uri string) ([]Object, error)
}

var (
	// ErrUnsupportedScheme is returned by For when the URI's scheme has no backend.
	ErrUnsupportedScheme = errors.New("storage: unsupported scheme")

	// ErrInvalidPath is returned for file:// URIs that fail path validation
	// (missing scheme path, relative path, or a "." / ".." segment).
	ErrInvalidPath = errors.New("storage: invalid path")

	// ErrNotExist indicates that the addressed object does not exist. It
	// aliases io/fs.ErrNotExist so callers can use errors.Is with either
	// sentinel without importing backend-specific error types.
	ErrNotExist = fs.ErrNotExist

	storesMu sync.Mutex
	stores   = map[string]Store{}
)

// resetForTest clears the memoized Store map. Exposed for tests in this
// package; do not call from production code.
func resetForTest() {
	storesMu.Lock()
	defer storesMu.Unlock()
	stores = map[string]Store{}
}

// For returns the Store that handles the URI's scheme. The returned Store
// is memoized per scheme so backends can reuse clients across calls.
func For(uri string) (Store, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("storage: parse %q: %w", uri, err)
	}
	storesMu.Lock()
	defer storesMu.Unlock()
	if s, ok := stores[u.Scheme]; ok {
		return s, nil
	}
	var s Store
	switch u.Scheme {
	case "gs":
		s = &gcsStore{}
	case "file":
		s = &fileStore{}
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedScheme, u.Scheme)
	}
	stores[u.Scheme] = s
	return s, nil
}

// Get reads the object at uri.
func Get(ctx context.Context, uri string) ([]byte, error) {
	s, err := For(uri)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, uri)
}

// PutFile uploads the contents of source to uri.
func PutFile(ctx context.Context, uri, source string) error {
	s, err := For(uri)
	if err != nil {
		return err
	}
	return s.PutFile(ctx, uri, source)
}

// PutBytes writes data to uri, recording contentType as metadata.
func PutBytes(ctx context.Context, uri string, data []byte, contentType string) error {
	s, err := For(uri)
	if err != nil {
		return err
	}
	return s.PutBytes(ctx, uri, data, contentType)
}

// Delete removes the object at uri.
func Delete(ctx context.Context, uri string) error {
	s, err := For(uri)
	if err != nil {
		return err
	}
	return s.Delete(ctx, uri)
}

// List returns all objects under uri as a recursive, lexicographically
// ordered slice.
func List(ctx context.Context, uri string) ([]Object, error) {
	s, err := For(uri)
	if err != nil {
		return nil, err
	}
	return s.List(ctx, uri)
}
