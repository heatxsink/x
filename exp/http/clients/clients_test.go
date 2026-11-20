package clients

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// countingServer returns a server that counts accepted TCP connections.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var conns atomic.Int64
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv, &conns
}

func hammer(t *testing.T, client *http.Client, url string, workers, rounds int) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Go(func() {
			for range rounds {
				req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
				if err != nil {
					errs <- err
					return
				}
				resp, err := client.Do(req)
				if err != nil {
					errs <- err
					return
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("request failed: %v", err)
	}
}

func TestDefaultReusesConnectionsPerHost(t *testing.T) {
	const workers, rounds = 16, 50
	srv, conns := countingServer(t)
	client := Default()
	defer client.CloseIdleConnections()

	hammer(t, client, srv.URL, workers, rounds)

	// With the net/http default of 2 idle conns per host this dials hundreds
	// of connections. A correctly sized pool needs about one per worker; allow
	// slack for a request racing a connection's return to the pool.
	if got := conns.Load(); got > 2*workers {
		t.Fatalf("dialed %d connections for %d concurrent workers; idle pool is not retaining per-host connections", got, workers)
	}
}

func TestProxyTransportSettings(t *testing.T) {
	client, err := Proxy("http://127.0.0.1:3128")
	if err != nil {
		t.Fatalf("Proxy: %v", err)
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is %T, want *http.Transport", client.Transport)
	}
	if tr.MaxIdleConnsPerHost != MaxIdleConnsPerHost {
		t.Fatalf("MaxIdleConnsPerHost = %d, want %d", tr.MaxIdleConnsPerHost, MaxIdleConnsPerHost)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	u, err := tr.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy func: %v", err)
	}
	if u == nil || u.Host != "127.0.0.1:3128" {
		t.Fatalf("proxy URL = %v, want 127.0.0.1:3128", u)
	}
}

func TestProxyInvalidURL(t *testing.T) {
	if _, err := Proxy("://bad"); err == nil {
		t.Fatal("expected error for invalid proxy URL")
	}
}
