package clients

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"
)

// MaxIdleConnsPerHost is the number of idle keep-alive connections retained
// per host. net/http defaults to 2, which on HTTP/1.1 forces every concurrent
// request beyond the second to dial a fresh connection and leave a socket in
// TIME_WAIT, exhausting ephemeral ports under load. Matching MaxIdleConns lets
// a single-host workload reuse the whole pool.
const MaxIdleConnsPerHost = 100

// Default returns a client with bounded timeouts and a connection pool sized
// for concurrent requests to a single host. Callers should create it once and
// reuse it; a new client per request defeats connection reuse entirely.
func Default() *http.Client {
	return newClient(nil)
}

// Proxy returns a client like Default that routes requests through addr.
func Proxy(addr string) (*http.Client, error) {
	proxyURL, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	return newClient(http.ProxyURL(proxyURL)), nil
}

func newClient(proxy func(*http.Request) (*url.URL, error)) *http.Client {
	transport := &http.Transport{
		Proxy: proxy,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   MaxIdleConnsPerHost,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}
