package healthz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// envelope mirrors the shape responses.OK wraps around the payload.
type envelope struct {
	StatusCode int      `json:"status_code"`
	StatusText string   `json:"status_text"`
	Data       *Healthz `json:"data"`
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var got envelope
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Data == nil {
		t.Fatalf("data field missing from response: %s", w.Body.String())
	}
	return got
}

func TestServeHTTP_PopulatesFields(t *testing.T) {
	h := &Healthz{
		Version:   "v1.2.3",
		BuildDate: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
		Hash:      "abc1234",
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	got := decodeBody(t, w)
	if got.Data.Version != "v1.2.3" || got.Data.Hash != "abc1234" {
		t.Errorf("Version/Hash = %q/%q, want v1.2.3/abc1234", got.Data.Version, got.Data.Hash)
	}
	if got.Data.TimeSince == "" {
		t.Errorf("TimeSince empty; want non-empty for valid BuildDate")
	}
}

func TestServeHTTP_AcceptsRFC3339(t *testing.T) {
	h := &Healthz{BuildDate: "2026-05-01T11:14:33Z"}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	got := decodeBody(t, w)
	if got.Data.TimeSince == "" {
		t.Errorf("RFC3339 BuildDate should produce a non-empty TimeSince")
	}
}

func TestServeHTTP_AcceptsLegacyFormat(t *testing.T) {
	h := &Healthz{BuildDate: "2026-05-01T11:14:33-0700"}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	got := decodeBody(t, w)
	if got.Data.TimeSince == "" {
		t.Errorf("legacy BuildDate should produce a non-empty TimeSince")
	}
}

func TestServeHTTP_UnparsableBuildDateLeavesTimeSinceEmpty(t *testing.T) {
	h := &Healthz{BuildDate: "garbage"}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	got := decodeBody(t, w)
	if got.Data.TimeSince != "" {
		t.Errorf("TimeSince = %q, want empty for unparsable BuildDate", got.Data.TimeSince)
	}
}

// TestServeHTTP_DoesNotMutateReceiver guards the race fix: prior to the
// rewrite the handler wrote h.TimeSince on every request. After: h is
// observed unchanged across requests.
func TestServeHTTP_DoesNotMutateReceiver(t *testing.T) {
	h := &Healthz{
		Version:   "v1.0.0",
		BuildDate: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
	}
	if h.TimeSince != "" {
		t.Fatalf("precondition: TimeSince should start empty, got %q", h.TimeSince)
	}

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	}
	if h.TimeSince != "" {
		t.Errorf("TimeSince mutated to %q; handler must not write to receiver", h.TimeSince)
	}
}

// TestServeHTTP_ConcurrentRequests is the explicit race regression. Run
// with `go test -race`. Prior to the rewrite, concurrent handlers raced
// on h.TimeSince.
func TestServeHTTP_ConcurrentRequests(t *testing.T) {
	h := &Healthz{
		Version:   "v1.0.0",
		BuildDate: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	}

	var wg sync.WaitGroup
	const n = 64
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", w.Code)
			}
		}()
	}
	wg.Wait()
}

func TestSatisfiesHTTPHandler(t *testing.T) {
	var _ http.Handler = (*Healthz)(nil)
}

func TestResponseHandler_ReturnsReceiver(t *testing.T) {
	h := &Healthz{Version: "v1"}
	got := h.ResponseHandler()
	hh, ok := got.(*Healthz)
	if !ok {
		t.Fatalf("ResponseHandler returned %T, want *Healthz", got)
	}
	if hh != h {
		t.Errorf("ResponseHandler did not return the receiver")
	}
}

func TestParseBuildDate_ErrorContainsInput(t *testing.T) {
	_, err := parseBuildDate("not-a-time")
	if err == nil {
		t.Fatal("expected error for unparsable input, got nil")
	}
	if !strings.Contains(err.Error(), "not-a-time") {
		t.Errorf("error %q should contain offending input", err)
	}
}
