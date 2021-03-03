package server

import (
	"fmt"
	"net/http"
)

func (s *Server) handleClientID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.frontendURL)

		_, err := w.Write([]byte(s.scdl.ClientID()))
		if err != nil {
			fmt.Println(err)
		}
	}
}
