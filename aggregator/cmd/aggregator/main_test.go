package main_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"learning-go/aggregator/internal/aggregator"
)

type handlerRoundTripper struct {
	handler http.Handler
}

func (roundTripper handlerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	responseRecorder := httptest.NewRecorder()
	roundTripper.handler.ServeHTTP(responseRecorder, request)

	if err := request.Context().Err(); err != nil {
		return nil, err
	}

	response := responseRecorder.Result()
	// Ensure the body is readable even after rr is GC'd.
	responseBytes, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(responseBytes))
	return response, nil
}

func TestAggregate_AlbumArtTimeoutStillReturnsOtherData(test *testing.T) {
	serverMux := http.NewServeMux()

	serverMux.HandleFunc("/api/artist-bio", func(respnoseWriter http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(respnoseWriter).Encode(map[string]any{"artist": "A", "bio": "B"})
	})
	serverMux.HandleFunc("/api/current-song", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{"artist": "A", "title": "Song"})
	})

	var canceled atomic.Bool
	serverMux.HandleFunc("/api/album-art", func(responseWriter http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
			canceled.Store(true)
			return
		case <-time.After(2 * time.Second):
			_ = json.NewEncoder(responseWriter).Encode(map[string]any{"url": "slow"})
		}
	})

	client := aggregator.NewClient("http://downstream.invalid")
	client.HTTP.Transport = handlerRoundTripper{handler: serverMux}
	handler := aggregator.NewHandler(client)
	handler.AlbumArtTimeout = 50 * time.Millisecond

	start := time.Now()
	request := httptest.NewRequest(http.MethodGet, "http://aggregator.local/aggregate", nil)
	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)
	response := responseRecorder.Result()

	if took := time.Since(start); took > 500*time.Millisecond {
		test.Fatalf("expected quick response, took %s", took)
	}

	var out aggregator.Response
	if err := json.NewDecoder(response.Body).Decode(&out); err != nil {
		test.Fatalf("decode response: %v", err)
	}

	if out.ArtistBio == nil || out.CurrentSong == nil {
		test.Fatalf("expected artist_bio and current_song, got %+v", out)
	}
	if out.AlbumArt != nil {
		test.Fatalf("expected album_art to be missing, got %+v", out.AlbumArt)
	}
	if len(out.Errors) == 0 {
		test.Fatalf("expected errors to include album_art timeout")
	}

	// Give the downstream handler a moment to observe cancellation.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && !canceled.Load() {
		time.Sleep(5 * time.Millisecond)
	}
	if !canceled.Load() {
		test.Fatalf("expected album art request to be canceled")
	}
}

func TestAggregate_AllSuccess(test *testing.T) {
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/api/artist-bio", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{"artist": "A", "bio": "B"})
	})
	serverMux.HandleFunc("/api/current-song", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{"artist": "A", "title": "Song"})
	})
	serverMux.HandleFunc("/api/album-art", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{"url": "ok"})
	})

	client := aggregator.NewClient("http://downstream.invalid")
	client.HTTP.Transport = handlerRoundTripper{handler: serverMux}
	handler := aggregator.NewHandler(client)
	handler.AlbumArtTimeout = 500 * time.Millisecond
	request := httptest.NewRequest(http.MethodGet, "http://aggregator.local/aggregate", nil)
	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)
	response := responseRecorder.Result()

	var out aggregator.Response
	if err := json.NewDecoder(response.Body).Decode(&out); err != nil {
		test.Fatalf("decode response: %v", err)
	}
	if out.ArtistBio == nil || out.CurrentSong == nil || out.AlbumArt == nil {
		test.Fatalf("expected all fields, got %+v", out)
	}
	if len(out.Errors) != 0 {
		test.Fatalf("expected no errors, got %v", out.Errors)
	}
}
