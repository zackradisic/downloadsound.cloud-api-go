package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

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

	scdl, err := soundcloudapi.New("")
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

	s.addRoute(s.router, "POST", "/track", s.handleTrack())
	s.addRoute(s.router, "POST", "/playlist", s.handlePlaylist())
}

func (s *Server) addRoute(router *mux.Router, method string, path string, handler func(http.ResponseWriter, *http.Request)) {
	router.HandleFunc(path, handler).Methods(method)
}

type getMediaURLResponse struct {
	URL string `json:"url"`
}

type failedRequestError struct {
	status int
	errMsg string
}

type errResponse struct {
	Err string `json:"err"`
}

func (f *failedRequestError) Error() string {
	if f.errMsg == "" {
		return fmt.Sprintf("Request returned non 2xx status: %d", f.status)
	}

	return fmt.Sprintf("Request failed with status %d: %s", f.status, f.errMsg)
}

type trackInfo struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	HLS   bool   `json:"hls"`
}

func (s *Server) getIMGURL(url string) string {
	if url == "" {
		return ""
	}

	end := strings.LastIndex(url, "-")
	if end == -1 {
		return url + "-t500x500.jpg"
	}
	return string([]rune(url)[0:strings.LastIndex(url, "-")]) + "-t500x500.jpg"
}

func (s *Server) getMediaURL(url string) (string, error) {
	res, err := http.Get(url + "?client_id=" + s.scdl.ClientID())
	if err != nil {
		if data, err := ioutil.ReadAll(res.Body); err == nil {
			return "", &failedRequestError{status: res.StatusCode, errMsg: string(data)}
		}
		return "", &failedRequestError{status: res.StatusCode}
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		if data, err := ioutil.ReadAll(res.Body); err == nil {
			return "", &failedRequestError{status: res.StatusCode, errMsg: string(data)}
		}
		return "", &failedRequestError{status: res.StatusCode}
	}

	data, err := ioutil.ReadAll(res.Body)

	body := &getMediaURLResponse{}

	err = json.Unmarshal(data, body)

	if err != nil {
		return "", errors.New("Invalid request body")
	}

	if body.URL == "" {
		return "", errors.New("Invalid request body")
	}

	return body.URL, nil
}

func (s *Server) getMediaURLMany(urls []trackInfo) ([]trackInfo, error) {
	type result struct {
		url   string
		index int
	}
	resChan := make(chan result, len(urls))
	errChan := make(chan error)

	for i, d := range urls {
		go func(i int, url string) {
			mediaURL, err := s.getMediaURL(url)
			if err != nil {
				errChan <- err
				return
			}
			resChan <- result{url: mediaURL, index: i}
		}(i, d.URL)
	}

	count := 0
	for {
		select {
		case err := <-errChan:
			return nil, err
		case res := <-resChan:
			urls[res.index].URL = res.url
			count++
		}

		if count == len(urls) {
			break
		}
	}

	return urls, nil
}

func (s *Server) handleTrack() http.HandlerFunc {
	type requestBody struct {
		URL string `json:"url"`
	}

	type responseBody struct {
		URL      string             `json:"url"`
		Title    string             `json:"title"`
		Author   soundcloudapi.User `json:"author"`
		ImageURL string             `json:"imageURL"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.frontendURL)

		decoder := json.NewDecoder(r.Body)
		body := &requestBody{}
		err := decoder.Decode(body)

		if err != nil {
			s.respondError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if !soundcloudapi.IsURL(body.URL) {
			s.respondError(w, "URL is not a track", http.StatusUnprocessableEntity)
			return
		}

		fmt.Println(body.URL)

		track, err := s.scdl.GetTrackInfo(soundcloudapi.GetTrackInfoOptions{URL: body.URL})

		if failedRequest, ok := err.(*soundcloudapi.FailedRequestError); ok {
			fmt.Printf("%d: %s\n", failedRequest.Status, failedRequest.ErrMsg)
			if failedRequest.Status == 404 {
				s.respondError(w, "Could not find that track.", failedRequest.Status)
				return
			}

			s.respondError(w, "", failedRequest.Status)
			return
		}

		if err != nil {
			fmt.Println(err.Error())
			s.respondError(w, "Internal server error occurred", http.StatusInternalServerError)
			return
		}

		url := ""

		if len(track[0].Media.Transcodings) == 0 {
			s.respondError(w, fmt.Sprintf("The track '%s' cannot be downloaded due to copyright.\n", track[0].Title), http.StatusBadRequest)
			return
		}

		for _, transcoding := range track[0].Media.Transcodings {
			if transcoding.Format.Protocol == "progressive" {
				url = transcoding.URL
			}
		}

		if url == "" {
			url = track[0].Media.Transcodings[0].URL
		}

		mediaURL, err := s.getMediaURL(url)

		if failedRequest, ok := err.(*failedRequestError); ok {
			fmt.Printf("%d: %s\n", failedRequest.status, failedRequest.errMsg)
			if failedRequest.status == 404 {
				s.respondError(w, "Could not finds that track.", failedRequest.status)
				return
			}

			s.respondJSON(w, failedRequest.errMsg, failedRequest.status)
			return
		}

		if err != nil {
			fmt.Println(err.Error())
			s.respondError(w, "Internal server error occurred", http.StatusInternalServerError)
		}

		imageURL := s.getIMGURL(track[0].ArtworkURL)
		if imageURL == "" {
			imageURL = s.getIMGURL(track[0].User.AvatarURL)
		}

		s.respondJSON(w, &responseBody{URL: mediaURL, Title: track[0].Title, Author: track[0].User, ImageURL: imageURL}, http.StatusOK)
	}
}

func (s *Server) handlePlaylist() http.HandlerFunc {
	type requestBody struct {
		URL string `json:"url"`
	}

	type responseBody struct {
		URL               string             `json:"url"`
		Title             string             `json:"title"`
		Tracks            []trackInfo        `json:"tracks"`
		CopyrightedTracks []string           `json:"copyrightedTracks"`
		Author            soundcloudapi.User `json:"author"`
		ImageURL          string             `json:"imageURL"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.frontendURL)

		decoder := json.NewDecoder(r.Body)
		body := &requestBody{}
		err := decoder.Decode(body)
		errRes := &errResponse{}

		if err != nil {
			s.respondError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if !soundcloudapi.IsURL(body.URL) || !strings.Contains(body.URL, "/sets/") {
			s.respondError(w, "URL is not a playlist", http.StatusUnprocessableEntity)
			return
		}

		fmt.Println(body.URL)

		playlist, err := s.scdl.GetPlaylistInfo(body.URL)

		if failedRequest, ok := err.(*failedRequestError); ok {
			fmt.Printf("%d: %s\n", failedRequest.status, failedRequest.errMsg)
			if failedRequest.status == 404 {
				s.respondError(w, "Could not find that playlist.", http.StatusNotFound)
				return
			}

			s.respondJSON(w, &errRes, failedRequest.status)
			return
		}

		if err != nil {
			fmt.Println(err.Error())
			s.respondError(w, "An internal server error occurred", http.StatusInternalServerError)
			return
		}

		copyrightedTracks := []string{}
		urls := []trackInfo{}

		for _, track := range playlist.Tracks {

			link := ""
			hls := true
			for _, transcoding := range track.Media.Transcodings {
				if transcoding.Format.Protocol == "progressive" {
					link = transcoding.URL
					hls = false
					break
				}
			}

			if len(track.Media.Transcodings) == 0 {
				copyrightedTracks = append(copyrightedTracks, track.Title)
				continue
			}

			if link == "" {
				link = track.Media.Transcodings[0].URL
			}

			if strings.Contains(link, "/preview/") {
				copyrightedTracks = append(copyrightedTracks, track.Title)
				// urls = append(urls, "")
			} else {
				urls = append(urls, trackInfo{Title: track.Title, HLS: hls, URL: link})
			}
		}

		mediaURLs, err := s.getMediaURLMany(urls)

		if failedRequest, ok := err.(*failedRequestError); ok {
			fmt.Printf("%d: %s\n", failedRequest.status, failedRequest.errMsg)
			if failedRequest.status == 404 {
				s.respondError(w, "Could not find one of the tracks in the playlist.", failedRequest.status)
				return
			}

			s.respondJSON(w, failedRequest.errMsg, failedRequest.status)
			return
		}

		if err != nil {
			fmt.Println(err.Error())
			s.respondError(w, "Internal server error occurred", http.StatusInternalServerError)
			return
		}

		imageURL := s.getIMGURL(playlist.ArtworkURL)
		if imageURL == "" {
			imageURL = s.getIMGURL(playlist.User.AvatarURL)
		}

		s.respondJSON(w, &responseBody{
			URL:               body.URL,
			Title:             playlist.Title,
			Tracks:            mediaURLs,
			CopyrightedTracks: copyrightedTracks,
			Author:            playlist.User,
			ImageURL:          imageURL}, http.StatusOK)

	}
}

func (s *Server) respondJSON(w http.ResponseWriter, payload interface{}, status int) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(payload)
	// response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(buffer.Bytes())
}

// respondError makes the error response with payload as json format
func (s *Server) respondError(w http.ResponseWriter, message string, status int) {
	s.respondJSON(w, &errResponse{Err: message}, status)
}

// Run runs the server
func (s *Server) Run(host string) {
	fmt.Println("Running server on " + host)
	log.Fatal(http.ListenAndServe(host, s.router))
}
