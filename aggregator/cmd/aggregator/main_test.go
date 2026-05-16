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
	h http.Handler
}

func (rt handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rr := httptest.NewRecorder()
	rt.h.ServeHTTP(rr, req)

	if err := req.Context().Err(); err != nil {
		return nil, err
	}

	res := rr.Result()
	// Ensure the body is readable even after rr is GC'd.
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	res.Body = io.NopCloser(bytes.NewReader(b))
	return res, nil
}

func TestAggregate_AlbumArtTimeoutStillReturnsOtherData(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/artist-bio", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"artist": "A", "bio": "B"})
	})
	mux.HandleFunc("/api/current-song", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"artist": "A", "title": "Song"})
	})

	var canceled atomic.Bool
	mux.HandleFunc("/api/album-art", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			canceled.Store(true)
			return
		case <-time.After(2 * time.Second):
			_ = json.NewEncoder(w).Encode(map[string]any{"url": "slow"})
		}
	})

	client := aggregator.NewClient("http://downstream.invalid")
	client.HTTP.Transport = handlerRoundTripper{h: mux}
	h := aggregator.NewHandler(client)
	h.AlbumArtTimeout = 50 * time.Millisecond

	start := time.Now()
	req := httptest.NewRequest(http.MethodGet, "http://aggregator.local/aggregate", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	resp := rr.Result()

	if took := time.Since(start); took > 500*time.Millisecond {
		t.Fatalf("expected quick response, took %s", took)
	}

	var out aggregator.Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if out.ArtistBio == nil || out.CurrentSong == nil {
		t.Fatalf("expected artist_bio and current_song, got %+v", out)
	}
	if out.AlbumArt != nil {
		t.Fatalf("expected album_art to be missing, got %+v", out.AlbumArt)
	}
	if len(out.Errors) == 0 {
		t.Fatalf("expected errors to include album_art timeout")
	}

	// Give the downstream handler a moment to observe cancellation.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && !canceled.Load() {
		time.Sleep(5 * time.Millisecond)
	}
	if !canceled.Load() {
		t.Fatalf("expected album art request to be canceled")
	}
}

func TestAggregate_AllSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/artist-bio", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"artist": "A", "bio": "B"})
	})
	mux.HandleFunc("/api/current-song", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"artist": "A", "title": "Song"})
	})
	mux.HandleFunc("/api/album-art", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"url": "ok"})
	})

	client := aggregator.NewClient("http://downstream.invalid")
	client.HTTP.Transport = handlerRoundTripper{h: mux}
	h := aggregator.NewHandler(client)
	h.AlbumArtTimeout = 500 * time.Millisecond
	req := httptest.NewRequest(http.MethodGet, "http://aggregator.local/aggregate", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	resp := rr.Result()

	var out aggregator.Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ArtistBio == nil || out.CurrentSong == nil || out.AlbumArt == nil {
		t.Fatalf("expected all fields, got %+v", out)
	}
	if len(out.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", out.Errors)
	}
}
