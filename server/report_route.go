package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) handleReport() http.HandlerFunc {
	type responseBody struct {
		URL          string `json:"url"`
		DownloadType string `json:"downloadType"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.frontendURL)

		decoder := json.NewDecoder(r.Body)
		body := &responseBody{}

		err := decoder.Decode(body)
		if err != nil {
			s.respondError(w, "Invalid body", http.StatusBadRequest)
			return
		}

		fmt.Println(Entry{
			Severity:  "NOTICE",
			Message:   fmt.Sprintf("TYPE: %s URL: %s", body.DownloadType, body.URL),
			Component: "report-link",
			Trace:     "downloadsoundcloud",
		})
	}
}
