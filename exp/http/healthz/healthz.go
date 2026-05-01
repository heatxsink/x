package healthz

import (
	"fmt"
	"net/http"
	"time"

	"github.com/heatxsink/x/exp/http/responses"
)

// Healthz is a small JSON probe describing what is currently running.
// It satisfies http.Handler; register it with mux.Handle directly.
type Healthz struct {
	Version   string `json:"version"`
	BuildDate string `json:"build_date"`
	TimeSince string `json:"time_since"`
	Hash      string `json:"commit_hash"`
}

// buildDateFormats lists the timestamp layouts the handler will accept
// for BuildDate, in priority order. RFC3339 is the canonical form;
// the second entry is retained for callers still emitting the legacy
// pre-RFC3339 layout (no colon in the timezone offset).
var buildDateFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z0700",
}

func parseBuildDate(s string) (time.Time, error) {
	for _, layout := range buildDateFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("healthz: unrecognised BuildDate %q", s)
}

// ServeHTTP renders Healthz as a JSON response. The handler builds a
// fresh response value per request rather than mutating h, so concurrent
// requests do not race on TimeSince.
func (h *Healthz) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	resp := Healthz{
		Version:   h.Version,
		BuildDate: h.BuildDate,
		Hash:      h.Hash,
	}
	if t, err := parseBuildDate(h.BuildDate); err == nil {
		resp.TimeSince = time.Since(t).String()
	}
	responses.OK(w, resp)
}

// ResponseHandler returns h.
//
// Deprecated: register *Healthz directly as an http.Handler instead.
func (h *Healthz) ResponseHandler() http.Handler { return h }
