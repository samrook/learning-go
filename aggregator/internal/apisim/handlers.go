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

func Register(serverMux *http.ServeMux, cfg Config) {
	serverMux.Handle("/api/artist-bio", delayHandler(cfg.ArtistBioDelay, func(responseWriter http.ResponseWriter, request *http.Request) any {
		return map[string]any{"artist": "The Go Gophers", "bio": "A totally real band used for demo data."}
	}))

	serverMux.Handle("/api/current-song", delayHandler(cfg.CurrentSongDelay, func(responseWriter http.ResponseWriter, request *http.Request) any {
		return map[string]any{"artist": "The Go Gophers", "title": "Context Switching Blues"}
	}))

	serverMux.Handle("/api/album-art", delayHandler(cfg.AlbumArtDelay, func(responseWriter http.ResponseWriter, request *http.Request) any {
		return map[string]any{"url": "https://example.com/album-art.png"}
	}))
}

func delayHandler(defaultDelay time.Duration, payload func(responseWriter http.ResponseWriter, request *http.Request) any) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		delay := defaultDelay
		if query := request.URL.Query().Get("delay_ms"); query != "" {
			if ms, err := strconv.Atoi(query); err == nil && ms >= 0 {
				delay = time.Duration(ms) * time.Millisecond
			}
		}

		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-request.Context().Done():
			return
		case <-timer.C:
		}

		responseWriter.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(responseWriter).Encode(payload(responseWriter, request))
	})
}
