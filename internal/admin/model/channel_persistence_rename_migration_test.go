package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRenameChannelPersistenceTablesPreservesRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		"CREATE TABLE channels (id TEXT PRIMARY KEY, name TEXT NOT NULL)",
		"CREATE INDEX idx_channels_status ON channels (name)",
		"INSERT INTO channels (id, name) VALUES ('platform-id', 'openai')",
		"CREATE TABLE personal_provider_connections (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, credential_encrypted TEXT NOT NULL)",
		"CREATE INDEX idx_personal_provider_user_name ON personal_provider_connections (user_id)",
		"INSERT INTO personal_provider_connections (id, user_id, credential_encrypted) VALUES ('personal-id', 'owner-id', 'ciphertext')",
		"CREATE TABLE personal_model_routes (user_id TEXT NOT NULL, model TEXT NOT NULL, route_policy TEXT NOT NULL)",
		"INSERT INTO personal_model_routes (user_id, model, route_policy) VALUES ('owner-id', 'gpt-test', 'personal_only')",
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("execute %q: %v", statement, err)
		}
	}

	err = db.Transaction(renameChannelPersistenceTables)
	if err != nil {
		t.Fatalf("rename tables: %v", err)
	}

	checks := []struct {
		legacy string
		target string
		query  string
		want   string
	}{
		{"channels", "platform_channels", "SELECT name FROM platform_channels WHERE id = 'platform-id'", "openai"},
		{"personal_provider_connections", PersonalChannelsTableName, "SELECT credential_encrypted FROM personal_channels WHERE id = 'personal-id'", "ciphertext"},
		{"personal_model_routes", PersonalChannelModelRoutesTableName, "SELECT route_policy FROM personal_channel_model_routes WHERE user_id = 'owner-id'", "personal_only"},
	}
	for _, check := range checks {
		if db.Migrator().HasTable(check.legacy) {
			t.Errorf("legacy table %s still exists", check.legacy)
		}
		if !db.Migrator().HasTable(check.target) {
			t.Errorf("renamed table %s does not exist", check.target)
		}
		var value string
		if err := db.Raw(check.query).Scan(&value).Error; err != nil {
			t.Errorf("read renamed table %s: %v", check.target, err)
		} else if value != check.want {
			t.Errorf("value in %s = %q, want %q", check.target, value, check.want)
		}
	}
	if !db.Migrator().HasIndex("platform_channels", "idx_platform_channels_status") {
		t.Error("platform channel status index was not renamed")
	}
	if !db.Migrator().HasIndex("personal_channels", "idx_personal_channel_user_name") {
		t.Error("personal channel owner index was not renamed")
	}
}

func TestRenameTableIfNeededRejectsAmbiguousState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec("CREATE TABLE channels (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := db.Exec("CREATE TABLE platform_channels (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create target table: %v", err)
	}
	if err := renameTableIfNeeded(db, "channels", "platform_channels"); err == nil {
		t.Fatal("expected migration to reject an ambiguous old/new table state")
	}
}

func TestChannelPersistenceModelsUseExplicitTableNames(t *testing.T) {
	if got := (PlatformChannel{}).TableName(); got != "platform_channels" {
		t.Fatalf("PlatformChannel table name = %q", got)
	}
	if got := (PersonalChannel{}).TableName(); got != "personal_channels" {
		t.Fatalf("PersonalChannel table name = %q", got)
	}
	if got := (PersonalChannelModelRoute{}).TableName(); got != "personal_channel_model_routes" {
		t.Fatalf("PersonalChannelModelRoute table name = %q", got)
	}
}
