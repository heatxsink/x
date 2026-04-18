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
)

const sidecarSuffix = ".meta.json"

type fileStore struct{}

type sidecar struct {
	ContentType string `json:"content_type,omitempty"`
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
		return "", fmt.Errorf("%w: non-empty host %q in %q", ErrInvalidPath, u.Host, uri)
	}
	if u.Path == "" {
		return "", fmt.Errorf("%w: empty path in %q", ErrInvalidPath, uri)
	}
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == ".." {
			return "", fmt.Errorf("%w: %q", ErrInvalidPath, uri)
		}
	}
	p := filepath.Clean(filepath.FromSlash(u.Path))
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%w: not absolute: %q", ErrInvalidPath, uri)
	}
	return p, nil
}

func (fileStore) Get(_ context.Context, uri string) ([]byte, error) {
	p, err := pathFromURI(uri)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("storage: get %q: %w", uri, err)
	}
	return data, nil
}

func (fileStore) PutFile(_ context.Context, uri, source string) error {
	p, err := pathFromURI(uri)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("storage: mkdir for %q: %w", uri, err)
	}
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("storage: open source %q: %w", source, err)
	}
	defer func() { _ = src.Close() }()
	dst, err := os.Create(p)
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
	return nil
}

func (fileStore) PutBytes(_ context.Context, uri string, data []byte, contentType string) error {
	p, err := pathFromURI(uri)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("storage: mkdir for %q: %w", uri, err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("storage: write %q: %w", uri, err)
	}
	if contentType != "" {
		meta, err := json.Marshal(sidecar{ContentType: contentType})
		if err != nil {
			return fmt.Errorf("storage: marshal sidecar for %q: %w", uri, err)
		}
		if err := os.WriteFile(p+sidecarSuffix, meta, 0o644); err != nil {
			return fmt.Errorf("storage: write sidecar for %q: %w", uri, err)
		}
	}
	return nil
}

func (fileStore) Delete(_ context.Context, uri string) error {
	p, err := pathFromURI(uri)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		return fmt.Errorf("storage: delete %q: %w", uri, err)
	}
	if err := os.Remove(p + sidecarSuffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storage: delete sidecar for %q: %w", uri, err)
	}
	return nil
}

func (fileStore) List(_ context.Context, uri string) ([]Object, error) {
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
		objs = append(objs, Object{
			URI:         "file://" + filepath.ToSlash(p),
			Size:        info.Size(),
			ContentType: readSidecar(p),
			Updated:     info.ModTime(),
		})
		return nil
	})
	if errors.Is(walkErr, fs.ErrNotExist) {
		return nil, nil
	}
	if walkErr != nil {
		return nil, fmt.Errorf("storage: list %q: %w", uri, walkErr)
	}
	sort.Slice(objs, func(i, j int) bool { return objs[i].URI < objs[j].URI })
	return objs, nil
}

func readSidecar(path string) string {
	data, err := os.ReadFile(path + sidecarSuffix)
	if err != nil {
		return ""
	}
	var s sidecar
	if err := json.Unmarshal(data, &s); err != nil {
		return ""
	}
	return s.ContentType
}

var _ Store = (*fileStore)(nil)
