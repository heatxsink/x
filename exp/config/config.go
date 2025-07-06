package config

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"cloud.google.com/go/storage"
)

func FromURI(ctx context.Context, uri string) ([]byte, error) {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "file":
		return FromFile(u.Path)
	case "gs":
		return FromGCS(ctx, u.Host, strings.TrimPrefix(u.Path, "/"))
	case "secret":
		return FromSecretManager(ctx, strings.TrimPrefix(u.Path, "/"))
	}
	return nil, fmt.Errorf("uri '%s' is not supported", u.Scheme)
}

func FromFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, f)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func FromGCS(ctx context.Context, bucket string, key string) ([]byte, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	bh := client.Bucket(bucket)
	o := bh.Object(key)
	buf := bytes.NewBuffer(nil)
	r, err := o.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	_, err = io.Copy(buf, r)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func FromSecretManager(ctx context.Context, resource string) ([]byte, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}
	defer client.Close()
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: resource,
	}
	resp, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to access secret '%s': %w", resource, err)
	}
	return resp.Payload.Data, nil
}
