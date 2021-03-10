package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	soundcloudapi "github.com/zackradisic/soundcloud-api"
)

func (s *Server) handleLikes() http.HandlerFunc {
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

		fmt.Println(body.URL)

		user, err := s.scdl.GetUser(soundcloudapi.GetUserOptions{ProfileURL: strings.TrimRight(body.URL, "/")})

		if failedRequest, ok := err.(*soundcloudapi.FailedRequestError); ok {
			if failedRequest.Status == 404 {
				s.respondError(w, "Couldn't find that user", 404)
				return
			}

			s.respondError(w, failedRequest.ErrMsg, failedRequest.Status)
			return
		}

		if err != nil {
			fmt.Println(err.Error())
			s.respondError(w, "Internal server error occurred", http.StatusInternalServerError)
			return
		}

		options := soundcloudapi.GetLikesOptions{
			ID:    user.ID,
			Limit: user.Likes,
			Type:  "track",
		}

		copyrightedTracks := []string{}
		urls := []trackInfo{}
		artworkURL := ""

		likeS := make([]soundcloudapi.Like, user.Likes)
		if user.Likes <= 200 {
			// TODO: Should clean this up, error checking is getting a bit ridiculous.
			// Here is a great pattern to use: https://thingsthatkeepmeupatnight.dev/posts/golang-http-handler-errors/
			likes, err := s.scdl.GetLikes(options)
			if failedRequest, ok := err.(*soundcloudapi.FailedRequestError); ok {
				if failedRequest.Status == 404 {
					s.respondError(w, "Couldn't find that user", 404)
					return
				}

				s.respondError(w, failedRequest.ErrMsg, failedRequest.Status)
				return
			}

			if err != nil {
				s.respondError(w, "Internal server error occurred", http.StatusInternalServerError)
				return
			}
			likeS, err = likes.GetLikes()
			if err != nil {
				s.respondError(w, "Internal server error occurred", http.StatusInternalServerError)
				return
			}
		} else {
			options.Limit = 1000
			err = s.getLikesBulk(r.Context(), &likeS, options)
			if err != nil {
				s.respondError(w, "Internal server error occurred", http.StatusInternalServerError)
			}
		}

		if failedRequest, ok := err.(*soundcloudapi.FailedRequestError); ok {
			if failedRequest.Status == 404 {
				s.respondError(w, "Couldn't find that user", 404)
				return
			}

			s.respondError(w, failedRequest.ErrMsg, failedRequest.Status)
			return
		}

		if err != nil {
			fmt.Println(err.Error())
			s.respondError(w, "Internal server error occurred", http.StatusInternalServerError)
			return
		}

		// TODO: This is identical in both here and the playlist route.
		// It can definitely be abstracted into its own function.
		for _, like := range likeS {
			if like.Track.Kind != "track" {
				continue
			}

			link := ""
			hls := true
			for _, transcoding := range like.Track.Media.Transcodings {
				if transcoding.Format.Protocol == "progressive" {
					link = transcoding.URL
					hls = false
					break
				}
			}

			if like.Track.Downloadable {
				hls = true
			}

			if len(like.Track.Media.Transcodings) == 0 {
				copyrightedTracks = append(copyrightedTracks, like.Track.Title)
				continue
			}

			if link == "" {
				link = like.Track.Media.Transcodings[0].URL
			}

			if strings.Contains(link, "/preview/") {
				copyrightedTracks = append(copyrightedTracks, like.Track.Title)
				// urls = append(urls, "")
			} else {
				urls = append(urls, trackInfo{Title: like.Track.Title, HLS: hls, URL: like.Track.PermalinkURL, Author: like.Track.User.Username})
				if like.Track.ArtworkURL != "" && artworkURL == "" {
					artworkURL = like.Track.ArtworkURL
				}
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

		imageURL := s.getIMGURL(user.AvatarURL)
		if imageURL == "" {
			imageURL = s.getIMGURL(artworkURL)
		}

		s.respondJSON(w, &responseBody{
			URL:               body.URL,
			Title:             fmt.Sprintf("%s's Likes", user.Username),
			Tracks:            mediaURLs,
			CopyrightedTracks: copyrightedTracks,
			Author:            user,
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
