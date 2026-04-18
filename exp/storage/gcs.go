package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"

	gcssdk "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type gcsStore struct {
	once    sync.Once
	client  *gcssdk.Client
	initErr error
}

func (g *gcsStore) getClient(ctx context.Context) (*gcssdk.Client, error) {
	g.once.Do(func() {
		g.client, g.initErr = gcssdk.NewClient(ctx)
	})
	if g.initErr != nil {
		return nil, g.initErr
	}
	return g.client, nil
}

func splitGS(uri string) (bucket, key string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}
	if u.Scheme != "gs" {
		return "", "", fmt.Errorf("expected gs scheme, got %q", u.Scheme)
	}
	if u.Host == "" {
		return "", "", errors.New("missing bucket")
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}

func (g *gcsStore) Get(ctx context.Context, uri string) ([]byte, error) {
	bucket, key, err := splitGS(uri)
	if err != nil {
		return nil, fmt.Errorf("storage: parse %q: %w", uri, err)
	}
	client, err := g.getClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: gcs client: %w", err)
	}
	r, err := client.Bucket(bucket).Object(key).NewReader(ctx)
	if err != nil {
		if errors.Is(err, gcssdk.ErrObjectNotExist) {
			return nil, fmt.Errorf("storage: get %q: %w", uri, ErrNotExist)
		}
		return nil, fmt.Errorf("storage: get %q: %w", uri, err)
	}
	defer func() { _ = r.Close() }()
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, r); err != nil {
		return nil, fmt.Errorf("storage: read %q: %w", uri, err)
	}
	return buf.Bytes(), nil
}

func (g *gcsStore) PutFile(ctx context.Context, uri, source string) error {
	bucket, key, err := splitGS(uri)
	if err != nil {
		return fmt.Errorf("storage: parse %q: %w", uri, err)
	}
	client, err := g.getClient(ctx)
	if err != nil {
		return fmt.Errorf("storage: gcs client: %w", err)
	}
	f, err := os.Open(source) // #nosec G304 -- source is a caller-supplied local path
	if err != nil {
		return fmt.Errorf("storage: open source %q: %w", source, err)
	}
	defer func() { _ = f.Close() }()
	w := client.Bucket(bucket).Object(key).NewWriter(ctx)
	if _, err := io.Copy(w, f); err != nil {
		_ = w.Close()
		return fmt.Errorf("storage: write %q: %w", uri, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("storage: close %q: %w", uri, err)
	}
	return nil
}

func (g *gcsStore) PutBytes(ctx context.Context, uri string, data []byte, contentType string) error {
	bucket, key, err := splitGS(uri)
	if err != nil {
		return fmt.Errorf("storage: parse %q: %w", uri, err)
	}
	client, err := g.getClient(ctx)
	if err != nil {
		return fmt.Errorf("storage: gcs client: %w", err)
	}
	w := client.Bucket(bucket).Object(key).NewWriter(ctx)
	w.ContentType = contentType
	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		_ = w.Close()
		return fmt.Errorf("storage: write %q: %w", uri, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("storage: close %q: %w", uri, err)
	}
	return nil
}

func (g *gcsStore) Delete(ctx context.Context, uri string) error {
	bucket, key, err := splitGS(uri)
	if err != nil {
		return fmt.Errorf("storage: parse %q: %w", uri, err)
	}
	client, err := g.getClient(ctx)
	if err != nil {
		return fmt.Errorf("storage: gcs client: %w", err)
	}
	if err := client.Bucket(bucket).Object(key).Delete(ctx); err != nil {
		if errors.Is(err, gcssdk.ErrObjectNotExist) {
			return fmt.Errorf("storage: delete %q: %w", uri, ErrNotExist)
		}
		return fmt.Errorf("storage: delete %q: %w", uri, err)
	}
	return nil
}

func (g *gcsStore) List(ctx context.Context, uri string) ([]Object, error) {
	bucket, prefix, err := splitGS(uri)
	if err != nil {
		return nil, fmt.Errorf("storage: parse %q: %w", uri, err)
	}
	client, err := g.getClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: gcs client: %w", err)
	}
	var q *gcssdk.Query
	if prefix != "" {
		q = &gcssdk.Query{Prefix: prefix}
	}
	var objs []Object
	it := client.Bucket(bucket).Objects(ctx, q)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("storage: list %q: %w", uri, err)
		}
		objs = append(objs, Object{
			URI:         "gs://" + bucket + "/" + attrs.Name,
			Size:        attrs.Size,
			ContentType: attrs.ContentType,
			Updated:     attrs.Updated,
		})
	}
	return objs, nil
}

var _ Store = (*gcsStore)(nil)
