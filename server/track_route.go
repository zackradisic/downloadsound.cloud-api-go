package server

import (
	"fmt"
	"net/http"

	soundcloudapi "github.com/zackradisic/soundcloud-api"
)

func (s *Server) handleTrack() http.HandlerFunc {
	type responseBody struct {
		URL      string             `json:"url"`
		Title    string             `json:"title"`
		Author   soundcloudapi.User `json:"author"`
		ImageURL string             `json:"imageURL"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.frontendURL)

		var body *urlRequestBody
		if valueRaw := r.Context().Value(ContextBody); valueRaw != nil {
			var ok bool
			body, ok = valueRaw.(*urlRequestBody)
			if !ok {
				s.respondError(w, "Invalid request body", http.StatusBadRequest)
			}
		} else {
			s.respondError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// TODO: Use a logger instead of just printing the URL here
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

		// Profile links will pass detection
		if track[0].Kind != "track" {
			desired := "PLAYLIST"
			if track[0].Kind == "user" {
				desired = "LIKES"
			}
			s.respondError(w, fmt.Sprintf("That isn't a track url! (hint: switch to the '%s' tab 👉)", desired), http.StatusBadRequest)
			return
		}

		if len(track[0].Media.Transcodings) == 0 {
			s.respondError(w, fmt.Sprintf("The track '%s' cannot be downloaded due to copyright.\n", track[0].Title), http.StatusBadRequest)
			return
		}

		mediaURL, err := s.scdl.GetDownloadURL(body.URL, "progressive")

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
