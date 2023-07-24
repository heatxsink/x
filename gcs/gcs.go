package gcs

import (
	"bytes"
	"context"
	"fmt"
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

func PutFile(ctx context.Context, bucket string, key string, source string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	bh := client.Bucket(bucket)
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	o := bh.Object(key)
	w := o.NewWriter(ctx)
	_, err = io.Copy(w, f)
	if err != nil {
		return err
	}
	defer w.Close()
	return nil
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
	iii := make([]*storage.ObjectAttrs, 0)
	client, err := storage.NewClient(ctx)
	if err != nil {
		return iii, err
	}
	bh := client.Bucket(bucket)
	ii := bh.Objects(ctx, nil)
	for {
		attrs, err := ii.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Println(err)
		}
		iii = append(iii, attrs)
	}
	return iii, nil
}
