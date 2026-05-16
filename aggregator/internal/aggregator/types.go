package aggregator

type ArtistBio struct {
	Artist string `json:"artist"`
	Bio    string `json:"bio"`
}

type CurrentSong struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
}

type AlbumArt struct {
	URL string `json:"url"`
}

type Response struct {
	ArtistBio   *ArtistBio   `json:"artist_bio,omitempty"`
	CurrentSong *CurrentSong `json:"current_song,omitempty"`
	AlbumArt    *AlbumArt    `json:"album_art,omitempty"`
	Errors      []string     `json:"errors,omitempty"`
}
