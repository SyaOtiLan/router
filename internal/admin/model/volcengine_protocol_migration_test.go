package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRenameVolcengineRealtimeChannelsToVolcengineWithDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&PlatformChannel{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	baseURL := "https://openspeech.bytedance.com"
	rows := []PlatformChannel{
		{Id: "realtime-1", Name: "realtime-1", Protocol: "volcengine-realtime", BaseURL: &baseURL},
		{Id: "standard-1", Name: "standard-1", Protocol: "volcengine", BaseURL: &baseURL},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed channels: %v", err)
	}

	if err := renameVolcengineRealtimeChannelsToVolcengineWithDB(db); err != nil {
		t.Fatalf("renameVolcengineRealtimeChannelsToVolcengineWithDB returned error: %v", err)
	}

	var channels []PlatformChannel
	if err := db.Order("id asc").Find(&channels).Error; err != nil {
		t.Fatalf("load channels: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("channel count = %d, want 2", len(channels))
	}
	for _, row := range channels {
		if row.Protocol != "volcengine" {
			t.Fatalf("channel %s protocol = %q, want volcengine", row.Id, row.Protocol)
		}
	}
}
