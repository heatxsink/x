package healthz

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/heatxsink/x/exp/response"
)

type Healthz struct {
	Version   string `json:"version"`
	BuildDate string `json:"build_date"`
	TimeSince string `json:"time_since"`
	Hash      string `json:"commit_hash"`
}

func (h *Healthz) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, _ := time.Parse("2006-01-02T15:04:05Z0700", h.BuildDate)
		h.TimeSince = time.Since(t).String()
		body, _ := json.MarshalIndent(&h, "", "  ")
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})
}

func (h *Healthz) ResponseHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, _ := time.Parse("2006-01-02T15:04:05Z0700", h.BuildDate)
		h.TimeSince = time.Since(t).String()
		response.OK(w, &h)
	})
}
