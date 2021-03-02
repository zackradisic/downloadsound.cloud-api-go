package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	soundcloudapi "github.com/zackradisic/soundcloud-api"
)

type urlRequestBody struct {
	URL string `json:"url"`
}

func (s *Server) validateLink(link linkType, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		decoder := json.NewDecoder(r.Body)
		body := &urlRequestBody{}
		err := decoder.Decode(body)

		if err != nil {
			s.respondError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if soundcloudapi.IsFirebaseURL(body.URL) {
			url, err := soundcloudapi.ConvertFirebaseLink(body.URL)
			if err != nil {
				s.respondError(w, "Invalid URL", http.StatusUnprocessableEntity)
				return
			}

			body.URL = url
		} else if soundcloudapi.IsMobileURL(body.URL) {
			body.URL = soundcloudapi.StripMobilePrefix(body.URL)
		} else if soundcloudapi.IsSearchURL(body.URL) {
			u, err := url.Parse(body.URL)
			if err != nil {
				s.respondError(w, "Invalid URL", http.StatusBadRequest)
				return
			}

			query := u.Query().Get("q")
			response, err := s.scdl.Search(soundcloudapi.SearchOptions{
				Query: query,
				Limit: 1,
				Kind:  soundcloudapi.KindTrack,
			})

			data, err := json.Marshal(response)
			pgQuery := &soundcloudapi.PaginatedQuery{}

			err = json.Unmarshal(data, pgQuery)
			if err != nil {
				s.respondError(w, "Invalid URL", http.StatusBadRequest)
				return
			}

			track, err := pgQuery.GetTracks()
			if err != nil || len(track) == 0 {
				s.respondError(w, "Invalid URL", http.StatusBadRequest)
				return
			}

			body.URL = track[0].PermalinkURL
		}

		switch link {
		case linkTypeTrack:
			if !s.scdl.IsURL(body.URL) {
				s.respondError(w, "URL is not a track", http.StatusUnprocessableEntity)
				return
			}

			if soundcloudapi.IsPlaylistURL(body.URL) {
				s.respondError(w, "URL is a playlist not a track", http.StatusBadRequest)
				return
			}
			break
		case linkTypePlaylist:
			if !s.scdl.IsURL(body.URL) || !soundcloudapi.IsPlaylistURL(body.URL) {
				s.respondError(w, "URL is not a playlist", http.StatusUnprocessableEntity)
				return
			}
			break
		case linkTypeLikes:
			if !s.scdl.IsURL(body.URL) {
				s.respondError(w, "URL is not a valid SoundCloud link", http.StatusUnprocessableEntity)
				return
			}
			break
		}

		ctx = context.WithValue(ctx, ContextBody, body)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
