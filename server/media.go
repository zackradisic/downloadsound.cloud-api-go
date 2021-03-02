package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

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
	Title  string `json:"title"`
	URL    string `json:"url"`
	HLS    bool   `json:"hls"`
	Author string `json:"author"`
}

// getIMGURL returns the URL to download the image specified by the given url.
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

// getMediaURL returns the URL to download the given SoundCloud resource
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

// getMediaURLMany modifies the given trackInfo array by fetching the
// media URL for each of its elements and setting its URL property
func (s *Server) getMediaURLMany(urls []trackInfo) ([]trackInfo, error) {
	if len(urls) == 0 {
		return nil, errors.New("No URLs provided")
	}
	type result struct {
		url   string
		index int
	}
	resChan := make(chan result, len(urls))
	errChan := make(chan error)

	for i, d := range urls {
		go func(i int, url string) {
			mediaURL, err := s.scdl.GetDownloadURL(url, "progressive")
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
