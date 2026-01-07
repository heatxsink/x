package gcs

import (
	"bytes"
	"context"
	"io"
	"os"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

func Get(ctx context.Context, bucket string, key string) ([]byte, error) {
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

func PutFile(ctx context.Context, bucket, key, source string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	w := client.Bucket(bucket).Object(key).NewWriter(ctx)
	if _, err = io.Copy(w, f); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func PutBytes(ctx context.Context, bucket string, key string, data []byte, contentType string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	bh := client.Bucket(bucket)
	o := bh.Object(key)
	w := o.NewWriter(ctx)
	w.ContentType = contentType
	w.CacheControl = "private, max-age=0, no-transform"
	defer w.Close()
	buf := bytes.NewBuffer(data)
	_, err = io.Copy(w, buf)
	return err
}

func Delete(ctx context.Context, bucket string, key string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	bh := client.Bucket(bucket)
	o := bh.Object(key)
	return o.Delete(ctx)
}

func List(ctx context.Context, bucket string) ([]*storage.ObjectAttrs, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	var objects []*storage.ObjectAttrs
	it := client.Bucket(bucket).Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return objects, err
		}
		objects = append(objects, attrs)
	}
	return objects, nil
}
