package model

import (
	"fmt"

	"gorm.io/gorm"
)

func CleanupDanglingGroupModelChannels() (int64, error) {
	return cleanupDanglingGroupModelChannelsWithDB(DB)
}

func cleanupDanglingGroupModelChannelsWithDB(db *gorm.DB) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is nil")
	}

	channelIDSubQuery := db.Model(&PlatformChannel{}).Select("id")
	result := db.
		Where("channel_id <> ''").
		Where("channel_id NOT IN (?)", channelIDSubQuery).
		Delete(&GroupModelChannel{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
