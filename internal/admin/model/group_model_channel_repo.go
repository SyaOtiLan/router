package model

import "context"

type GroupModelChannelRepository struct {
	GetRandomSatisfiedChannel                func(group string, model string, ignoreFirstPriority bool) (*ChannelCandidate, error)
	ListSatisfiedChannels                    func(group string, model string) ([]*ChannelCandidate, error)
	AddGroupModelChannels                    func(channel *ChannelCandidate) error
	DeleteGroupModelChannels                 func(channel *ChannelCandidate) error
	UpdateGroupModelChannels                 func(channel *ChannelCandidate) error
	RefreshGroupModelChannelsByChannelStatus func(channelId string, status bool) error
	GetTopChannelByModel                     func(group string, model string) (*ChannelCandidate, error)
	GetGroupModels                           func(ctx context.Context, group string) ([]string, error)
}

var groupModelChannelRepo GroupModelChannelRepository

func BindGroupModelChannelRepository(repo GroupModelChannelRepository) {
	groupModelChannelRepo = repo
}

func mustGroupModelChannelRepo() GroupModelChannelRepository {
	if groupModelChannelRepo.GetRandomSatisfiedChannel == nil {
		panic("group model channel repository not initialized")
	}
	if groupModelChannelRepo.ListSatisfiedChannels == nil {
		panic("group model channel repository not initialized")
	}
	return groupModelChannelRepo
}
