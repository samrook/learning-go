package apisim

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type Config struct {
	ArtistBioDelay   time.Duration
	CurrentSongDelay time.Duration
	AlbumArtDelay    time.Duration
}

func DefaultConfig() Config {
	return Config{
		ArtistBioDelay:   100 * time.Millisecond,
		CurrentSongDelay: 150 * time.Millisecond,
		AlbumArtDelay:    700 * time.Millisecond,
	}
}

func Register(mux *http.ServeMux, cfg Config) {
	mux.Handle("/api/artist-bio", delayHandler(cfg.ArtistBioDelay, func(w http.ResponseWriter, r *http.Request) any {
		return map[string]any{"artist": "The Go Gophers", "bio": "A totally real band used for demo data."}
	}))

	mux.Handle("/api/current-song", delayHandler(cfg.CurrentSongDelay, func(w http.ResponseWriter, r *http.Request) any {
		return map[string]any{"artist": "The Go Gophers", "title": "Context Switching Blues"}
	}))

	mux.Handle("/api/album-art", delayHandler(cfg.AlbumArtDelay, func(w http.ResponseWriter, r *http.Request) any {
		return map[string]any{"url": "https://example.com/album-art.png"}
	}))
}

func delayHandler(defaultDelay time.Duration, payload func(w http.ResponseWriter, r *http.Request) any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delay := defaultDelay
		if q := r.URL.Query().Get("delay_ms"); q != "" {
			if ms, err := strconv.Atoi(q); err == nil && ms >= 0 {
				delay = time.Duration(ms) * time.Millisecond
			}
		}

		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload(w, r))
	})
}
