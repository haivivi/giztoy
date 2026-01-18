package minimax

import (
	"context"
)

// VideoService provides video generation operations.
type VideoService struct {
	client *Client
}

// newVideoService creates a new video service.
func newVideoService(client *Client) *VideoService {
	return &VideoService{client: client}
}

// CreateTextToVideo creates a text-to-video generation task.
//
// Returns a Task that can be polled for completion.
//
// Example:
//
//	task, err := client.Video.CreateTextToVideo(ctx, &minimax.TextToVideoRequest{
//	    Model:  "MiniMax-Hailuo-02",
//	    Prompt: "A cat walking in a garden",
//	})
//	if err != nil {
//	    return err
//	}
//	result, err := task.Wait(ctx)
func (s *VideoService) CreateTextToVideo(ctx context.Context, req *TextToVideoRequest) (*Task[VideoResult], error) {
	var resp struct {
		TaskID   string    `json:"task_id"`
		BaseResp *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/video_generation", req, &resp)
	if err != nil {
		return nil, err
	}

	return &Task[VideoResult]{
		ID:       resp.TaskID,
		client:   s.client,
		taskType: taskTypeVideo,
	}, nil
}

// CreateImageToVideo creates an image-to-video generation task.
//
// The first frame image is used as the starting point for video generation.
func (s *VideoService) CreateImageToVideo(ctx context.Context, req *ImageToVideoRequest) (*Task[VideoResult], error) {
	var resp struct {
		TaskID   string    `json:"task_id"`
		BaseResp *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/video_generation", req, &resp)
	if err != nil {
		return nil, err
	}

	return &Task[VideoResult]{
		ID:       resp.TaskID,
		client:   s.client,
		taskType: taskTypeVideo,
	}, nil
}

// CreateFrameToVideo creates a first/last frame to video generation task.
//
// Both first and last frame images are required for generating the video.
func (s *VideoService) CreateFrameToVideo(ctx context.Context, req *FrameToVideoRequest) (*Task[VideoResult], error) {
	var resp struct {
		TaskID   string    `json:"task_id"`
		BaseResp *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/video_generation", req, &resp)
	if err != nil {
		return nil, err
	}

	return &Task[VideoResult]{
		ID:       resp.TaskID,
		client:   s.client,
		taskType: taskTypeVideo,
	}, nil
}

// CreateSubjectRefVideo creates a subject reference video generation task.
//
// The subject reference image is used to maintain subject consistency.
func (s *VideoService) CreateSubjectRefVideo(ctx context.Context, req *SubjectRefVideoRequest) (*Task[VideoResult], error) {
	var resp struct {
		TaskID   string    `json:"task_id"`
		BaseResp *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/video_generation", req, &resp)
	if err != nil {
		return nil, err
	}

	return &Task[VideoResult]{
		ID:       resp.TaskID,
		client:   s.client,
		taskType: taskTypeVideo,
	}, nil
}

// CreateAgentTask creates a video agent task using a template.
//
// Video Agent allows creating videos from predefined templates with
// customizable media and text inputs.
func (s *VideoService) CreateAgentTask(ctx context.Context, req *VideoAgentRequest) (*Task[VideoResult], error) {
	var resp struct {
		TaskID   string    `json:"task_id"`
		BaseResp *baseResp `json:"base_resp"`
	}

	err := s.client.http.request(ctx, "POST", "/v1/video_agent", req, &resp)
	if err != nil {
		return nil, err
	}

	return &Task[VideoResult]{
		ID:       resp.TaskID,
		client:   s.client,
		taskType: taskTypeVideo,
	}, nil
}
