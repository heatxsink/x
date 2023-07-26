package healthz

import (
	"encoding/json"
	"net/http"
)

type Healthz struct {
	Version   string `json:"version"`
	BuildDate string `json:"build_date"`
	Hash      string `json:"commit_hash"`
}

func (h *Healthz) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.MarshalIndent(&h, "", "  ")
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})
}
