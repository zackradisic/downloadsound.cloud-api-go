package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	soundcloudapi "github.com/zackradisic/soundcloud-api"
)

// Server is the REST API server
type Server struct {
	router      *mux.Router
	frontendURL string
	scdl        *soundcloudapi.API
}

// New returns a new server
func New() *Server {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		log.Fatal("frontendURL is required")
	}

	scdl, err := soundcloudapi.New(soundcloudapi.APIOptions{
		HTTPClient: &http.Client{
			Timeout: time.Second * 15,
		},
	})
	if err != nil {
		log.Fatal(err.Error())
	}

	s := &Server{
		router:      mux.NewRouter().StrictSlash(true),
		frontendURL: frontendURL,
		scdl:        scdl,
	}

	s.setupRoutes()

	return s
}

func (s *Server) setupPreflightRoutes() {
	s.router.Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.frontendURL)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Access-Control-Request-Headers, Access-Control-Request-Method, Connection, Host, Origin, User-Agent, Referer, Cache-Control, X-header")
		w.WriteHeader(http.StatusNoContent)
		return
	})
}

func (s *Server) setupRoutes() {
	s.setupPreflightRoutes()

	s.addRoute(s.router, "POST", "/track", s.validateLink(linkTypeTrack, s.handleTrack()))
	s.addRoute(s.router, "POST", "/playlist", s.validateLink(linkTypePlaylist, s.handlePlaylist()))
	s.addRoute(s.router, "POST", "/likes", s.validateLink(linkTypeLikes, s.handleLikes()))
}

func (s *Server) addRoute(router *mux.Router, method string, path string, handler func(http.ResponseWriter, *http.Request)) {
	router.HandleFunc(path, handler).Methods(method)
}

// respondError makes the error response with payload as json format
func (s *Server) respondError(w http.ResponseWriter, message string, status int) {
	s.respondJSON(w, &errResponse{Err: message}, status)
}

// Run runs the server
func (s *Server) Run(host string) {
	fmt.Println("Running server on " + host)
	srv := &http.Server{
		Addr:    host,
		Handler: http.TimeoutHandler(s.router, 15*time.Second, "Request timed out."),
	}
	log.Fatal(srv.ListenAndServe())
}
