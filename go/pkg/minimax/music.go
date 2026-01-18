package minimax

import (
	"context"
)

// MusicService provides music generation operations.
type MusicService struct {
	client *Client
}

// newMusicService creates a new music service.
func newMusicService(client *Client) *MusicService {
	return &MusicService{client: client}
}

// musicResponse is the API response for music generation.
type musicResponse struct {
	Data struct {
		Audio string `json:"audio"` // hex-encoded
	} `json:"data"`
	ExtraInfo *AudioInfo `json:"extra_info"`
	BaseResp  *baseResp  `json:"base_resp"`
}

// Generate generates music from prompt and lyrics.
//
// Currently supports generating music up to 1 minute in length.
//
// Example:
//
//	resp, err := client.Music.Generate(ctx, &minimax.MusicRequest{
//	    Prompt: "Pop music, happy mood, suitable for morning",
//	    Lyrics: "[Verse]\nHello world\nIt's a beautiful day\n[Chorus]\nLet's celebrate",
//	})
func (s *MusicService) Generate(ctx context.Context, req *MusicRequest) (*MusicResponse, error) {
	var apiResp musicResponse

	err := s.client.http.request(ctx, "POST", "/v1/music/generation", req, &apiResp)
	if err != nil {
		return nil, err
	}

	resp := &MusicResponse{
		ExtraInfo: apiResp.ExtraInfo,
	}

	// Calculate duration from extra_info if available
	if apiResp.ExtraInfo != nil {
		resp.Duration = apiResp.ExtraInfo.AudioLength
	}

	// Decode hex audio
	if apiResp.Data.Audio != "" {
		audio, err := decodeHexAudio(apiResp.Data.Audio)
		if err != nil {
			return nil, err
		}
		resp.Audio = audio
	}

	return resp, nil
}
