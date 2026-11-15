package model

type ChannelRepository struct {
	GetChannelById               func(id string) (*PlatformChannel, error)
	Insert                       func(channel *PlatformChannel) error
	Update                       func(channel *PlatformChannel) error
	UpdateModels                 func(id string, rows []ChannelModel) error
	UpdateResponseTime           func(channel *PlatformChannel, responseTime int64)
	Delete                       func(channel *PlatformChannel) error
	UpdateChannelStatusById      func(id string, status int) error
	UpdateChannelUsedQuota       func(id string, quota int64)
	UpdateChannelUsedQuotaDirect func(id string, quota int64)
	UpdateChannelTestModelByID   func(id string, testModel string) error
	DeleteChannelByStatus        func(status int64) (int64, error)
	DeleteDisabledChannel        func() (int64, error)
}

var channelRepo ChannelRepository

func BindChannelRepository(repo ChannelRepository) {
	channelRepo = repo
}

func mustChannelRepo() ChannelRepository {
	if channelRepo.GetChannelById == nil {
		panic("channel repository not initialized")
	}
	return channelRepo
}
