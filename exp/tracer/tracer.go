package tracer

import (
	"context"
	"fmt"
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

func Do(ctx context.Context, client *http.Client, req *http.Request) (*Tracer, error) {
	tt := &Tracer{
		URL:     req.URL.String(),
		Created: time.Now(),
	}
	trace := &httptrace.ClientTrace{
		DNSStart:             tt.DNSStart,
		DNSDone:              tt.DNSDone,
		ConnectStart:         tt.ConnectStart,
		GotConn:              tt.GotConn,
		GotFirstResponseByte: tt.GotFirstResponseByte,
	}
	tt.Trace = trace
	htctx := httptrace.WithClientTrace(ctx, tt.Trace)
	req = req.WithContext(htctx)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res != nil {
		tt.BodyReadTime = time.Now()
	}
	tt.HTTPResponse = res
	return tt, err
}
