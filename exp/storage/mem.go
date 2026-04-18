package storage

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// memStore is a process-global in-memory backend keyed by full URI.
//
// Test isolation: callers should include a unique namespace in the URI host
// (e.g., mem://<t.Name()>/key) because the backend is memoized per scheme
// for the lifetime of the process.
type memStore struct {
	mu      sync.RWMutex
	objects map[string]memObject
}

type memObject struct {
	data        []byte
	contentType string
	updated     time.Time
}

func newMemStore() *memStore {
	return &memStore{objects: map[string]memObject{}}
}

func memKey(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("storage: parse %q: %w", uri, err)
	}
	if u.Scheme != "mem" {
		return "", fmt.Errorf("%w: expected mem scheme, got %q", ErrInvalidURI, u.Scheme)
	}
	return (&url.URL{Scheme: "mem", Host: u.Host, Path: u.Path}).String(), nil
}

func (m *memStore) Get(ctx context.Context, uri string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	k, err := memKey(uri)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	obj, ok := m.objects[k]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("storage: get %q: %w", uri, ErrNotExist)
	}
	out := make([]byte, len(obj.data))
	copy(out, obj.data)
	return out, nil
}

func (m *memStore) PutFile(ctx context.Context, uri, source string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	k, err := memKey(uri)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source) // #nosec G304 -- source is a caller-supplied local path
	if err != nil {
		return fmt.Errorf("storage: open source %q: %w", source, err)
	}
	m.mu.Lock()
	m.objects[k] = memObject{data: data, updated: time.Now()}
	m.mu.Unlock()
	return nil
}

func (m *memStore) PutBytes(ctx context.Context, uri string, data []byte, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	k, err := memKey(uri)
	if err != nil {
		return err
	}
	stored := make([]byte, len(data))
	copy(stored, data)
	m.mu.Lock()
	m.objects[k] = memObject{data: stored, contentType: contentType, updated: time.Now()}
	m.mu.Unlock()
	return nil
}

func (m *memStore) Delete(ctx context.Context, uri string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	k, err := memKey(uri)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.objects[k]; !ok {
		return fmt.Errorf("storage: delete %q: %w", uri, ErrNotExist)
	}
	delete(m.objects, k)
	return nil
}

func (m *memStore) List(ctx context.Context, uri string) ([]Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prefix, err := memKey(uri)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Object, 0, len(m.objects))
	for k, obj := range m.objects {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		out = append(out, Object{
			URI:         k,
			Size:        int64(len(obj.data)),
			ContentType: obj.contentType,
			Updated:     obj.updated,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URI < out[j].URI })
	return out, nil
}

var _ Store = (*memStore)(nil)
