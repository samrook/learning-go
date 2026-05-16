package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Handler struct {
	Client *Client

	DefaultTimeout  time.Duration
	AlbumArtTimeout time.Duration
}

func NewHandler(client *Client) *Handler {
	return &Handler{
		Client:          client,
		DefaultTimeout:  2 * time.Second,
		AlbumArtTimeout: 500 * time.Millisecond,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parentCtx := r.Context()

	var (
		result Response
		mu     sync.Mutex
		wg     sync.WaitGroup
		errCh  = make(chan error, 3)
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(parentCtx, h.DefaultTimeout)
		defer cancel()

		val, err := h.Client.ArtistBio(ctx)
		if err != nil {
			errCh <- wrapErr("artist_bio", err)
			return
		}

		mu.Lock()
		result.ArtistBio = &val
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(parentCtx, h.DefaultTimeout)
		defer cancel()

		val, err := h.Client.CurrentSong(ctx)
		if err != nil {
			errCh <- wrapErr("current_song", err)
			return
		}

		mu.Lock()
		result.CurrentSong = &val
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(parentCtx, h.AlbumArtTimeout)
		defer cancel()

		val, err := h.Client.AlbumArt(ctx)
		if err != nil {
			errCh <- wrapErr("album_art", err)
			return
		}

		mu.Lock()
		result.AlbumArt = &val
		mu.Unlock()
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err == nil {
			continue
		}
		result.Errors = append(result.Errors, err.Error())
	}

	status := http.StatusOK
	if result.ArtistBio == nil && result.CurrentSong == nil && result.AlbumArt == nil {
		status = http.StatusBadGateway
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(result)
}

func wrapErr(component string, err error) error {
	return fmt.Errorf("%s: %w", component, err)
}
