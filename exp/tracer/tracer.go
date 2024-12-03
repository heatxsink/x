package tracer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"time"
)

type Tracer struct {
	URL                      string
	Created                  time.Time
	RequestStart             time.Time
	DNSStartTime             time.Time
	DNSDoneTime              time.Time
	GotConnTime              time.Time
	GotFirstResponseByteTime time.Time
	BodyReadTime             time.Time
	Trace                    *httptrace.ClientTrace
	HTTPResponse             *http.Response
}

func New(url string) *Tracer {
	tracer := &Tracer{
		URL:     url,
		Created: time.Now(),
	}
	trace := &httptrace.ClientTrace{
		DNSStart:             tracer.DNSStart,
		DNSDone:              tracer.DNSDone,
		ConnectStart:         tracer.ConnectStart,
		GotConn:              tracer.GotConn,
		GotFirstResponseByte: tracer.GotFirstResponseByte,
	}
	tracer.Trace = trace
	return tracer
}

type Result struct {
	URL              string
	DNSLookup        time.Duration
	TCPConnection    time.Duration
	ServerProcessing time.Duration
	ContentTransfer  time.Duration
	Total            time.Duration
}

func (r *Result) String() string {
	return fmt.Sprintf("%s {DNS: %v TCP: %v Server: %v Transfer: %v Total: %v}",
		r.URL, r.DNSLookup, r.TCPConnection, r.ServerProcessing, r.ContentTransfer, r.Total)
}

func (t *Tracer) GetResult() *Result {
	if t.RequestStart.IsZero() {
		t.RequestStart = t.Created
	}
	return &Result{
		URL:              t.URL,
		DNSLookup:        t.DNSDoneTime.Sub(t.DNSStartTime),
		TCPConnection:    t.GotConnTime.Sub(t.RequestStart),
		ServerProcessing: t.GotFirstResponseByteTime.Sub(t.GotConnTime),
		ContentTransfer:  t.BodyReadTime.Sub(t.GotFirstResponseByteTime),
		Total:            t.BodyReadTime.Sub(t.RequestStart),
	}
}

func (t *Tracer) DNSStart(_ httptrace.DNSStartInfo) {
	t.RequestStart = time.Now()
	t.DNSStartTime = time.Now()
}

func (t *Tracer) DNSDone(_ httptrace.DNSDoneInfo) {
	t.DNSDoneTime = time.Now()
}

func (t *Tracer) ConnectStart(_, _ string) {
	if t.RequestStart.IsZero() {
		t.RequestStart = time.Now()
	}
}

func (t *Tracer) GotFirstResponseByte() {
	t.GotFirstResponseByteTime = time.Now()
}

func (t *Tracer) GotConn(info httptrace.GotConnInfo) {
	t.GotConnTime = time.Now()
}

type TracerBodyReader struct {
	io.ReadCloser
	bodyReadTime *time.Time
}

func (tbr TracerBodyReader) Read(p []byte) (n int, err error) {
	n, err = tbr.ReadCloser.Read(p)
	if err != nil {
		*tbr.bodyReadTime = time.Now()
	}
	return
}

func DoRequest(ctx context.Context, client *http.Client, req *http.Request) (*Tracer, error) {
	tracer := New(req.URL.String())
	htctx := httptrace.WithClientTrace(ctx, tracer.Trace)
	req = req.WithContext(htctx)
	res, err := client.Do(req)
	if res != nil {
		res.Body = TracerBodyReader{
			ReadCloser:   res.Body,
			bodyReadTime: &tracer.BodyReadTime,
		}
	}
	tracer.HTTPResponse = res
	return tracer, err
}
