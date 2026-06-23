package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// ChannelService manages channel lifecycle and the assignment of account
// sources to channels.
type ChannelService struct {
	channels *repository.ChannelRepo
	sources  *repository.AccountSourceRepo
	audit    *repository.AuditRepo
}

// NewChannelService returns a ChannelService with the given dependencies.
func NewChannelService(channels *repository.ChannelRepo, sources *repository.AccountSourceRepo, audit *repository.AuditRepo) *ChannelService {
	return &ChannelService{channels: channels, sources: sources, audit: audit}
}

// Create creates a new channel and writes an audit entry.
func (s *ChannelService) Create(ctx context.Context, userID, name, description string) (*repository.Channel, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("create channel: name is required")
	}

	channel, err := s.channels.Create(ctx, name, description)
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      userID,
		Action:       "channel.created",
		ResourceType: "channel",
		ResourceID:   channel.ID,
	})
	return channel, nil
}

// List returns all channels.
func (s *ChannelService) List(ctx context.Context) ([]repository.Channel, error) {
	channels, err := s.channels.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return channels, nil
}

// AddSource associates an account source with a channel. Both the channel and
// the source must exist and be active.
func (s *ChannelService) AddSource(ctx context.Context, userID, channelID, sourceID string) error {
	if channelID == "" || sourceID == "" {
		return fmt.Errorf("add source to channel: channel_id and source_id are required")
	}

	// Verify the channel exists.
	channel, err := s.channels.FindByID(ctx, channelID)
	if err != nil {
		return fmt.Errorf("add source to channel: %w", err)
	}
	if channel == nil {
		return fmt.Errorf("add source to channel: channel %s not found", channelID)
	}

	// Verify the source exists and is active.
	source, err := s.sources.FindByID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("add source to channel: %w", err)
	}
	if source == nil {
		return fmt.Errorf("add source to channel: source %s not found", sourceID)
	}

	if err := s.channels.AddSource(ctx, channelID, sourceID); err != nil {
		return fmt.Errorf("add source to channel: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      userID,
		Action:       "channel.source_added",
		ResourceType: "channel",
		ResourceID:   channelID,
		MetadataJSON: fmt.Sprintf(`{"source_id":"%s"}`, sourceID),
	})
	return nil
}

// RemoveSource dissociates an account source from a channel.
func (s *ChannelService) RemoveSource(ctx context.Context, userID, channelID, sourceID string) error {
	if channelID == "" || sourceID == "" {
		return fmt.Errorf("remove source from channel: channel_id and source_id are required")
	}

	if err := s.channels.RemoveSource(ctx, channelID, sourceID); err != nil {
		return fmt.Errorf("remove source from channel: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      userID,
		Action:       "channel.source_removed",
		ResourceType: "channel",
		ResourceID:   channelID,
		MetadataJSON: fmt.Sprintf(`{"source_id":"%s"}`, sourceID),
	})
	return nil
}
