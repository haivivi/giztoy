package minimax

import (
	"context"
)

// ImageService provides image generation operations.
type ImageService struct {
	client *Client
}

// newImageService creates a new image service.
func newImageService(client *Client) *ImageService {
	return &ImageService{client: client}
}

// Generate generates images from text.
//
// Example:
//
//	resp, err := client.Image.Generate(ctx, &minimax.ImageGenerateRequest{
//	    Model:       "image-01",
//	    Prompt:      "A beautiful sunset over mountains",
//	    AspectRatio: "16:9",
//	    N:           1,
//	})
func (s *ImageService) Generate(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error) {
	var resp struct {
		Data struct {
			ImageURLs []string `json:"image_urls"`
		} `json:"data"`
		BaseResp *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/image_generation", req, &resp)
	if err != nil {
		return nil, err
	}

	// Convert URLs to ImageData
	images := make([]ImageData, len(resp.Data.ImageURLs))
	for i, url := range resp.Data.ImageURLs {
		images[i] = ImageData{URL: url}
	}

	return &ImageResponse{
		Images: images,
	}, nil
}

// GenerateWithReference generates images with a reference image.
//
// The reference image influences the generated output based on the
// ImagePromptStrength parameter (0-1).
func (s *ImageService) GenerateWithReference(ctx context.Context, req *ImageReferenceRequest) (*ImageResponse, error) {
	var resp struct {
		Data struct {
			ImageURLs []string `json:"image_urls"`
		} `json:"data"`
		BaseResp *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/image_generation", req, &resp)
	if err != nil {
		return nil, err
	}

	// Convert URLs to ImageData
	images := make([]ImageData, len(resp.Data.ImageURLs))
	for i, url := range resp.Data.ImageURLs {
		images[i] = ImageData{URL: url}
	}

	return &ImageResponse{
		Images: images,
	}, nil
}
