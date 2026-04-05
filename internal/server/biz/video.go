package biz

import (
	"context"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

type VideoService struct {
	ChannelService *ChannelService
	RequestService *RequestService
}

func NewVideoService(channelService *ChannelService, requestService *RequestService) *VideoService {
	return &VideoService{
		ChannelService: channelService,
		RequestService: requestService,
	}
}

func (s *VideoService) GetTask(ctx context.Context, requestID int) (*llm.Response, error) {
	task, ch, outbound, err := s.loadTask(ctx, requestID)
	if err != nil {
		return nil, err
	}

	httpReq, err := outbound.BuildGetVideoTaskRequest(ctx, task.ExternalID)
	if err != nil {
		return nil, err
	}

	httpResp, err := ch.HTTPClient.Do(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	video, err := outbound.ParseGetVideoTaskResponse(ctx, httpResp)
	if err != nil {
		return nil, err
	}

	// Persist snapshot to request table for task tracking.
	status := mapVideoStatusToRequestStatus(video.Video.Status)

	// Always store the latest response snapshot.
	if err := s.RequestService.UpdateRequestStatusExternalIDAndResponseBody(ctx, requestID, status, task.ExternalID, video.Video, nil); err != nil {
		// non-fatal: return data anyway
	}

	return video, nil
}

// GetTaskByExternalID looks up a video task by the provider's task ID (external_id).
// NOTE: assumes provider task IDs are globally unique across channels.
func (s *VideoService) GetTaskByExternalID(ctx context.Context, externalID string) (*llm.Response, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("%w: ent client not found in context", ErrInternal)
	}

	task, err := client.Request.Query().
		Where(request.ExternalID(externalID)).
		Only(ctx)
	if err != nil {
		return nil, err
	}

	return s.GetTask(ctx, task.ID)
}

// DeleteTaskByExternalID deletes a video task by the provider's task ID (external_id).
// NOTE: assumes provider task IDs are globally unique across channels.
func (s *VideoService) DeleteTaskByExternalID(ctx context.Context, externalID string) error {
	client := ent.FromContext(ctx)
	if client == nil {
		return fmt.Errorf("%w: ent client not found in context", ErrInternal)
	}

	task, err := client.Request.Query().
		Where(request.ExternalID(externalID)).
		Only(ctx)
	if err != nil {
		return err
	}

	return s.DeleteTask(ctx, task.ID)
}

func (s *VideoService) DeleteTask(ctx context.Context, requestID int) error {
	task, ch, outbound, err := s.loadTask(ctx, requestID)
	if err != nil {
		return err
	}

	httpReq, err := outbound.BuildDeleteVideoTaskRequest(ctx, task.ExternalID)
	if err != nil {
		return err
	}

	_, err = ch.HTTPClient.Do(ctx, httpReq)
	if err != nil {
		return err
	}

	// Best effort: mark canceled locally.
	_ = s.RequestService.UpdateRequestStatus(ctx, requestID, request.StatusCanceled)

	return nil
}

func (s *VideoService) loadTask(ctx context.Context, requestID int) (*ent.Request, *Channel, transformer.VideoTaskOutbound, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, nil, nil, fmt.Errorf("%w: ent client not found in context", ErrInternal)
	}

	task, err := client.Request.Get(ctx, requestID)
	if err != nil {
		return nil, nil, nil, err
	}

	if strings.TrimSpace(task.ExternalID) == "" {
		return nil, nil, nil, fmt.Errorf("%w: missing external_id for task", ErrInternal)
	}

	if task.ChannelID == 0 {
		return nil, nil, nil, fmt.Errorf("%w: missing channel_id for task", ErrInternal)
	}

	ch, err := s.ChannelService.GetChannel(ctx, task.ChannelID)
	if err != nil {
		return nil, nil, nil, err
	}

	outbound, ok := ch.Outbound.(transformer.VideoTaskOutbound)
	if !ok {
		return nil, nil, nil, fmt.Errorf("%w: channel does not support video task operations", ErrInternal)
	}

	return task, ch, outbound, nil
}

func mapVideoStatusToRequestStatus(status string) request.Status {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded":
		return request.StatusCompleted
	case "failed":
		return request.StatusFailed
	case "queued", "running":
		return request.StatusProcessing
	default:
		return request.StatusProcessing
	}
}
