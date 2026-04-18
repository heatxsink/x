// Package storage provides a URI-addressable blob store that dispatches
// between Google Cloud Storage (gs://bucket/key) and the local filesystem
// (file:///abs/path). Callers switch backends by changing a URI; the
// surface never exposes backend-specific types.
package storage

import (
	"context"
	"errors"
	"fmt"
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
	Get(ctx context.Context, uri string) ([]byte, error)
	PutFile(ctx context.Context, uri, source string) error
	PutBytes(ctx context.Context, uri string, data []byte, contentType string) error
	Delete(ctx context.Context, uri string) error
	List(ctx context.Context, uri string) ([]Object, error)
}

var (
	// ErrUnsupportedScheme is returned by For when the URI's scheme has no backend.
	ErrUnsupportedScheme = errors.New("storage: unsupported scheme")

	// ErrInvalidPath is returned for file:// URIs that fail path validation
	// (missing scheme path, relative path, or a ".." segment).
	ErrInvalidPath = errors.New("storage: invalid path")

	storesMu sync.Mutex
	stores   = map[string]Store{}
)

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
