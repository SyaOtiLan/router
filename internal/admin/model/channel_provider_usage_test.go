package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpsertChannelProviderUsageRecordIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:channel-provider-usage?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ChannelProviderUsageRecord{}, &ChannelProviderUsageSyncState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	first, err := UpsertChannelProviderUsageRecordWithDB(db, ChannelProviderUsageRecord{
		ChannelId: "channel-1", Adapter: "aixhan", UpstreamRecordId: "95547541", OccurredAt: 100, InputTokens: 10,
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	second, err := UpsertChannelProviderUsageRecordWithDB(db, ChannelProviderUsageRecord{
		ChannelId: "channel-1", Adapter: "aixhan", UpstreamRecordId: "95547541", OccurredAt: 101, InputTokens: 12,
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if first.Id != second.Id {
		t.Fatalf("identity changed: first=%q second=%q", first.Id, second.Id)
	}
	var count int64
	if err := db.Model(&ChannelProviderUsageRecord{}).Where("channel_id = ?", "channel-1").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("count=%d, want 1", count)
	}
	var stored ChannelProviderUsageRecord
	if err := db.First(&stored, "id = ?", first.Id).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if stored.OccurredAt != 101 || stored.InputTokens != 12 {
		t.Fatalf("stored=%+v", stored)
	}
}
