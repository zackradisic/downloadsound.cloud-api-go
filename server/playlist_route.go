package server

import (
	"fmt"
	"net/http"
	"strings"

	soundcloudapi "github.com/zackradisic/soundcloud-api"
)

func (s *Server) handlePlaylist() http.HandlerFunc {
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

		errRes := &errResponse{}
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

		// TODO: This is identical in both here and the likes route.
		// It can definitely be abstracted into its own function.
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

			if track.Downloadable {
				hls = false
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
				urls = append(urls, trackInfo{Title: track.Title, HLS: hls, URL: track.PermalinkURL, Author: track.User.Username})
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
			msg := "Internal server error occurred"
			if err.Error() == "No URLs provided" {
				s.respondError(w, "None of those tracks can be downloaded. (Likely due to copyright)", http.StatusConflict)
				return
			}
			s.respondError(w, msg, http.StatusInternalServerError)
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
