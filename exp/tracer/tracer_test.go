package tracer

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestDoRequest(t *testing.T) {
	ctx := context.Background()
	url := "https://www.google.com"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Error(err)
	}
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
	tt, err := Do(ctx, client, req)
	if err != nil {
		t.Error(err)
	}
	defer tt.Response.Body.Close()
	fmt.Println(tt.GetResult())
}
