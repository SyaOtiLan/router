package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLegacyChannelBillingSnapshotTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE channel_billing_snapshots (
			id char(36) PRIMARY KEY,
			channel_id char(36) NOT NULL,
			source_type varchar(32) NOT NULL DEFAULT 'manual',
			balance double precision NOT NULL DEFAULT 0,
			currency varchar(16) NOT NULL DEFAULT '',
			raw_status varchar(64) NOT NULL DEFAULT '',
			message text,
			request_url text,
			response_excerpt text,
			operator_user_id char(36) NOT NULL DEFAULT '',
			task_id char(36) NOT NULL DEFAULT '',
			created_at bigint NOT NULL DEFAULT 0
		)
	`).Error; err != nil {
		t.Fatalf("create legacy snapshot table: %v", err)
	}
	return db
}

func TestCreateChannelBillingSnapshotWithDBRepairsMissingPurchaseFields(t *testing.T) {
	db := newLegacyChannelBillingSnapshotTestDB(t)
	if db.Migrator().HasColumn(&ChannelBillingSnapshot{}, "PurchaseCostAmount") {
		t.Fatalf("legacy snapshot table unexpectedly has purchase_cost_amount")
	}

	row, err := CreateChannelBillingSnapshotWithDB(db, ChannelBillingSnapshot{
		Id:                 "snapshot-1",
		ChannelId:          "channel-1",
		SourceType:         ChannelBillingSnapshotSourceManual,
		Currency:           "USD",
		PurchaseCurrency:   "USD",
		PurchaseAmount:     10,
		PurchaseFXRate:     1,
		PurchaseCostAmount: 10,
		CreatedAt:          100,
	})
	if err != nil {
		t.Fatalf("CreateChannelBillingSnapshotWithDB returned error: %v", err)
	}
	if row.Id != "snapshot-1" {
		t.Fatalf("snapshot id = %q, want snapshot-1", row.Id)
	}
	if !db.Migrator().HasColumn(&ChannelBillingSnapshot{}, "PurchaseCostAmount") {
		t.Fatalf("purchase_cost_amount column was not repaired")
	}
	for _, column := range []string{"EventType", "ParentSnapshotId", "OldBatchDisposition"} {
		if !db.Migrator().HasColumn(&ChannelBillingSnapshot{}, column) {
			t.Fatalf("%s column was not repaired", column)
		}
	}
}

func TestUpdateChannelBillingSnapshotPurchaseWithDBRepairsMissingPurchaseFields(t *testing.T) {
	db := newLegacyChannelBillingSnapshotTestDB(t)
	if err := db.Exec(`
		INSERT INTO channel_billing_snapshots (
			id, channel_id, source_type, balance, currency, raw_status, message, request_url, response_excerpt, operator_user_id, task_id, created_at
		) VALUES (
			'snapshot-1', 'channel-1', 'manual', 0, 'USD', '', '', '', '', '', '', 100
		)
	`).Error; err != nil {
		t.Fatalf("seed legacy snapshot row: %v", err)
	}

	updated, err := UpdateChannelBillingSnapshotPurchaseWithDB(db, ChannelBillingSnapshot{
		Id:                 "snapshot-1",
		ChannelId:          "channel-1",
		PurchaseAt:         200,
		PurchaseCurrency:   "USD",
		PurchaseAmount:     12,
		PurchaseFXRate:     1,
		PurchaseCostAmount: 12,
		Message:            "manual update",
		OperatorUserId:     "user-1",
	})
	if err != nil {
		t.Fatalf("UpdateChannelBillingSnapshotPurchaseWithDB returned error: %v", err)
	}
	if updated.PurchaseCostAmount != 12 {
		t.Fatalf("purchase_cost_amount = %v, want 12", updated.PurchaseCostAmount)
	}
	if !db.Migrator().HasColumn(&ChannelBillingSnapshot{}, "PurchaseCostAmount") {
		t.Fatalf("purchase_cost_amount column was not repaired")
	}
}
