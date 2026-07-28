// Package storage provides a URI-addressable blob store that dispatches
// between Google Cloud Storage (gs://bucket/key), the local filesystem
// (file:///abs/path), and an in-memory backend (mem://namespace/key) for
// tests. Callers switch backends by changing a URI; the surface never
// exposes backend-specific types.
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

	// Generation and Metageneration are identifiers used for optimistic
	// concurrency. Generation changes on every object replacement and is
	// reported by all three backends; it matches what GetWithGeneration
	// returns for the same object. Metageneration changes on every metadata
	// update and is zero outside the gs:// backend.
	Generation     int64
	Metageneration int64
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
	// sidecar records the content type and the object's generation; an
	// empty contentType preserves any previously recorded content type.
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

// ConditionalStore extends Store with generation-preconditioned reads and
// writes for optimistic concurrency. All three built-in backends implement
// it; the extension interface keeps Store small so external implementations
// (test doubles, wrappers) keep compiling. Callers who work through URIs
// should use the package-level GetWithGeneration and PutBytesIfGeneration,
// which type-assert and return an error wrapping ErrUnsupported when the
// backend lacks conditional support.
type ConditionalStore interface {
	Store

	// GetWithGeneration reads the entire object at uri into memory and
	// returns its current generation for use as PutBytesIfGeneration's
	// ifGeneration. Returns an error wrapping ErrNotExist when the object
	// does not exist.
	GetWithGeneration(ctx context.Context, uri string) (data []byte, generation int64, err error)

	// PutBytesIfGeneration writes data to uri only if the object's current
	// generation matches ifGeneration; ifGeneration 0 requires that the
	// object not exist yet (create). Returns an error wrapping
	// ErrPreconditionFailed when the precondition does not hold, leaving
	// the stored object untouched. contentType is handled as in PutBytes.
	//
	// Generations are opaque: obtain them from GetWithGeneration (or List)
	// and compare only for equality. gs:// uses real GCS preconditions.
	// file:// persists a counter in the <path>.meta.json sidecar but only
	// serializes writers within one process; mem:// counts in memory. Both
	// are honest for their single-process dev and test roles only.
	PutBytesIfGeneration(ctx context.Context, uri string, data []byte, contentType string, ifGeneration int64) error
}

var (
	// ErrUnsupportedScheme is returned by For when the URI's scheme has no backend.
	ErrUnsupportedScheme = errors.New("storage: unsupported scheme")

	// ErrInvalidURI is returned when a URI fails validation — missing path,
	// missing bucket on gs://, non-absolute file:// path, or a "." / ".."
	// segment in a file:// path.
	ErrInvalidURI = errors.New("storage: invalid uri")

	// ErrNotExist indicates that the addressed object does not exist. It
	// aliases io/fs.ErrNotExist so callers can use errors.Is with either
	// sentinel without importing backend-specific error types.
	ErrNotExist = fs.ErrNotExist

	// ErrPreconditionFailed indicates that a conditional write lost the
	// race: the object's generation no longer matched the caller's
	// ifGeneration. Callers typically re-read with GetWithGeneration and
	// retry.
	ErrPreconditionFailed = errors.New("storage: precondition failed")

	// ErrUnsupported indicates that the backend does not implement the
	// requested optional capability, such as conditional writes. It aliases
	// errors.ErrUnsupported so callers can use errors.Is with either
	// sentinel.
	ErrUnsupported = errors.ErrUnsupported

	storesMu sync.RWMutex
	stores   = map[string]Store{}
)

// For returns the Store that handles the URI's scheme. The returned Store
// is memoized per scheme so backends can reuse clients across calls.
func For(uri string) (Store, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("storage: parse %q: %w", uri, err)
	}
	storesMu.RLock()
	s, ok := stores[u.Scheme]
	storesMu.RUnlock()
	if ok {
		return s, nil
	}
	storesMu.Lock()
	defer storesMu.Unlock()
	if s, ok := stores[u.Scheme]; ok {
		return s, nil
	}
	switch u.Scheme {
	case "gs":
		s = &gcsStore{}
	case "file":
		s = &fileStore{}
	case "mem":
		s = newMemStore()
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

// GetWithGeneration reads the object at uri and returns its current
// generation for use with PutBytesIfGeneration. Returns an error wrapping
// ErrUnsupported when the URI's backend does not implement ConditionalStore.
func GetWithGeneration(ctx context.Context, uri string) ([]byte, int64, error) {
	s, err := For(uri)
	if err != nil {
		return nil, 0, err
	}
	cs, ok := s.(ConditionalStore)
	if !ok {
		return nil, 0, fmt.Errorf("storage: get with generation %q: conditional reads: %w", uri, ErrUnsupported)
	}
	return cs.GetWithGeneration(ctx, uri)
}

// PutBytesIfGeneration writes data to uri only if the object's generation
// still matches ifGeneration (0 = the object must not exist yet). Returns an
// error wrapping ErrPreconditionFailed when another writer got there first,
// and an error wrapping ErrUnsupported when the URI's backend does not
// implement ConditionalStore.
func PutBytesIfGeneration(ctx context.Context, uri string, data []byte, contentType string, ifGeneration int64) error {
	s, err := For(uri)
	if err != nil {
		return err
	}
	cs, ok := s.(ConditionalStore)
	if !ok {
		return fmt.Errorf("storage: put %q: conditional writes: %w", uri, ErrUnsupported)
	}
	return cs.PutBytesIfGeneration(ctx, uri, data, contentType, ifGeneration)
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
