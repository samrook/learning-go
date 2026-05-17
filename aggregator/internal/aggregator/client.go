package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %s", response.Status)
	}

	return json.NewDecoder(response.Body).Decode(out)
}

func (c *Client) ArtistBio(ctx context.Context) (ArtistBio, error) {
	var out ArtistBio
	return out, c.getJSON(ctx, "/api/artist-bio", &out)
}

func (c *Client) CurrentSong(ctx context.Context) (CurrentSong, error) {
	var out CurrentSong
	return out, c.getJSON(ctx, "/api/current-song", &out)
}

func (c *Client) AlbumArt(ctx context.Context) (AlbumArt, error) {
	var out AlbumArt
	return out, c.getJSON(ctx, "/api/album-art", &out)
}
