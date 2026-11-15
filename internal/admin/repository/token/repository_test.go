package token

import (
	"testing"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTokenRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Token{}, &model.TokenGroupRoute{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	previousDB := model.DB
	previousRedisEnabled := common.RedisEnabled
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedisEnabled
	})
	return db
}

func TestUpdateInvalidatesTokenCache(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	if err := db.Create(&model.Token{
		Id:          "token-1",
		UserId:      "user-1",
		Key:         "secret-key-1",
		Name:        "before",
		Status:      model.TokenStatusEnabled,
		CreatedTime: 100,
		UpdatedTime: 100,
		Models:      stringPtr("gpt-4o-mini"),
	}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	var invalidatedKey string
	previousInvalidate := invalidateTokenCacheFn
	invalidateTokenCacheFn = func(key string) error {
		invalidatedKey = key
		return nil
	}
	t.Cleanup(func() {
		invalidateTokenCacheFn = previousInvalidate
	})

	token := &model.Token{
		Id:          "token-1",
		UserId:      "user-1",
		Key:         "secret-key-1",
		Name:        "after",
		Status:      model.TokenStatusEnabled,
		CreatedTime: 100,
		UpdatedTime: 200,
		Models:      stringPtr("gpt-4o-mini,gpt-4.1"),
	}
	if err := Update(token); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if invalidatedKey != "secret-key-1" {
		t.Fatalf("invalidated key=%q, want secret-key-1", invalidatedKey)
	}

	stored, err := GetByID("token-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Name != "after" {
		t.Fatalf("stored name=%q, want after", stored.Name)
	}
	if stored.Models == nil || *stored.Models != "gpt-4o-mini,gpt-4.1" {
		t.Fatalf("stored models=%v, want updated models", stored.Models)
	}
}

func TestDeleteInvalidatesTokenCache(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:          "token-1",
		UserId:      "user-1",
		Key:         "secret-key-1",
		Name:        "alpha",
		Status:      model.TokenStatusEnabled,
		CreatedTime: 100,
		UpdatedTime: 100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	if err := model.ReplaceTokenGroupRouteOrderWithDB(db, token.Id, []string{"group-a"}, 100); err != nil {
		t.Fatalf("seed token group order: %v", err)
	}

	var invalidatedKey string
	previousInvalidate := invalidateTokenCacheFn
	invalidateTokenCacheFn = func(key string) error {
		invalidatedKey = key
		return nil
	}
	t.Cleanup(func() {
		invalidateTokenCacheFn = previousInvalidate
	})

	if err := Delete(token); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if invalidatedKey != "secret-key-1" {
		t.Fatalf("invalidated key=%q, want secret-key-1", invalidatedKey)
	}

	if _, err := GetByID("token-1"); err == nil {
		t.Fatalf("expected deleted token lookup to fail")
	}
	if order, err := model.ListTokenGroupRouteOrderWithDB(db, "token-1"); err != nil || len(order) != 0 {
		t.Fatalf("deleted token group order=%v err=%v, want no rows", order, err)
	}
}

func TestCreateWithGroupRouteOrderIsAtomic(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		UserId:      "user-1",
		Key:         "secret-key-create-1",
		Name:        "created",
		Status:      model.TokenStatusEnabled,
		CreatedTime: 100,
		UpdatedTime: 100,
	}
	if err := CreateWithGroupRouteOrder(token, []string{"group-a", "group-b"}); err != nil {
		t.Fatalf("CreateWithGroupRouteOrder: %v", err)
	}
	if token.Id == "" {
		t.Fatal("expected create to assign a token ID")
	}
	order, err := model.ListTokenGroupRouteOrderWithDB(db, token.Id)
	if err != nil || len(order) != 2 || order[0] != "group-a" || order[1] != "group-b" {
		t.Fatalf("created token route order=%v err=%v, want [group-a group-b]", order, err)
	}

	failed := &model.Token{
		UserId:      "user-1",
		Key:         "secret-key-create-2",
		Name:        "must-rollback",
		Status:      model.TokenStatusEnabled,
		CreatedTime: 100,
		UpdatedTime: 100,
	}
	if err := CreateWithGroupRouteOrder(failed, []string{"group-a", "group-a"}); err == nil {
		t.Fatal("duplicate group order should fail")
	}
	if _, err := GetByID(failed.Id); err == nil {
		t.Fatal("token row should roll back when its group order fails")
	}
}

func TestUpdateWithGroupRouteOrderIsAtomic(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:          "token-1",
		UserId:      "user-1",
		Key:         "secret-key-1",
		Name:        "before",
		Status:      model.TokenStatusEnabled,
		CreatedTime: 100,
		UpdatedTime: 100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	if err := model.ReplaceTokenGroupRouteOrderWithDB(db, token.Id, []string{"group-a", "group-b"}, 100); err != nil {
		t.Fatalf("seed route order: %v", err)
	}

	previousInvalidate := invalidateTokenCacheFn
	invalidateTokenCacheFn = func(string) error { return nil }
	t.Cleanup(func() { invalidateTokenCacheFn = previousInvalidate })

	updated := *token
	updated.Name = "after"
	updated.UpdatedTime = 200
	if err := UpdateWithGroupRouteOrder(&updated, []string{"group-b", "group-a"}); err != nil {
		t.Fatalf("update token and order: %v", err)
	}
	stored, err := GetByID(token.Id)
	if err != nil {
		t.Fatalf("read updated token: %v", err)
	}
	if stored.Name != "after" {
		t.Fatalf("name=%q, want after", stored.Name)
	}
	order, err := model.ListTokenGroupRouteOrderWithDB(db, token.Id)
	if err != nil {
		t.Fatalf("read order: %v", err)
	}
	if len(order) != 2 || order[0] != "group-b" || order[1] != "group-a" {
		t.Fatalf("order=%v, want [group-b group-a]", order)
	}

	updated.Name = "must-rollback"
	updated.UpdatedTime = 300
	if err := UpdateWithGroupRouteOrder(&updated, []string{"group-a", "group-a"}); err == nil {
		t.Fatal("duplicate order should fail")
	}
	stored, err = GetByID(token.Id)
	if err != nil {
		t.Fatalf("read token after rejected update: %v", err)
	}
	if stored.Name != "after" {
		t.Fatalf("name after failed order update=%q, want rolled-back name after", stored.Name)
	}
	order, err = model.ListTokenGroupRouteOrderWithDB(db, token.Id)
	if err != nil {
		t.Fatalf("read order after rejected update: %v", err)
	}
	if len(order) != 2 || order[0] != "group-b" || order[1] != "group-a" {
		t.Fatalf("order after failed update=%v, want previous order [group-b group-a]", order)
	}
}

func TestValidateUserTokenRejectsExhaustedRequestCount(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:                    "token-1",
		UserId:                "user-1",
		Key:                   "secret-key-1",
		Name:                  "alpha",
		Status:                model.TokenStatusEnabled,
		ExpiredTime:           -1,
		RemainQuota:           1000,
		UnlimitedQuota:        false,
		RemainRequestCount:    0,
		UnlimitedRequestCount: false,
		CreatedTime:           100,
		UpdatedTime:           100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	_, err := ValidateUserToken("secret-key-1")
	if err == nil {
		t.Fatal("expected request-count exhausted token to be rejected")
	}
	if err.Error() != "该令牌请求次数已用尽" {
		t.Fatalf("ValidateUserToken error=%q, want request count exhausted", err.Error())
	}
}

func TestValidateUserTokenAllowsZeroMonetaryQuotaForPersonalRouting(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:                    "token-personal-only",
		UserId:                "user-1",
		Key:                   "personal-zero-quota-key",
		Name:                  "personal token",
		Status:                model.TokenStatusExhausted,
		ExpiredTime:           -1,
		RemainQuota:           0,
		UnlimitedQuota:        false,
		RemainRequestCount:    10,
		UnlimitedRequestCount: false,
		CreatedTime:           100,
		UpdatedTime:           100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	validated, err := ValidateUserToken(token.Key)
	if err != nil {
		t.Fatalf("ValidateUserToken() error = %v", err)
	}
	if validated == nil || validated.Id != token.Id {
		t.Fatalf("validated token = %#v", validated)
	}
	var stored model.Token
	if err := db.First(&stored, "id = ?", token.Id).Error; err != nil {
		t.Fatalf("load restored token: %v", err)
	}
	if stored.Status != model.TokenStatusEnabled {
		t.Fatalf("stored status = %d, want enabled", stored.Status)
	}
}

func TestConsumeTokenRequestCountDecrementsFiniteLimit(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:                    "token-1",
		UserId:                "user-1",
		Key:                   "secret-key-1",
		Name:                  "alpha",
		Status:                model.TokenStatusEnabled,
		RemainRequestCount:    3,
		UnlimitedRequestCount: false,
		UsedRequestCount:      2,
		CreatedTime:           100,
		UpdatedTime:           100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	if err := model.ConsumeTokenRequestCount("token-1", 2); err != nil {
		t.Fatalf("ConsumeTokenRequestCount: %v", err)
	}

	var stored model.Token
	if err := db.First(&stored, "id = ?", "token-1").Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if stored.RemainRequestCount != 1 {
		t.Fatalf("RemainRequestCount=%d, want 1", stored.RemainRequestCount)
	}
	if stored.UsedRequestCount != 4 {
		t.Fatalf("UsedRequestCount=%d, want 4", stored.UsedRequestCount)
	}
}

func TestConsumeTokenRequestCountRejectsInsufficientFiniteLimit(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:                    "token-1",
		UserId:                "user-1",
		Key:                   "secret-key-1",
		Name:                  "alpha",
		Status:                model.TokenStatusEnabled,
		RemainRequestCount:    1,
		UnlimitedRequestCount: false,
		UsedRequestCount:      2,
		CreatedTime:           100,
		UpdatedTime:           100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	if err := model.ConsumeTokenRequestCount("token-1", 2); err == nil {
		t.Fatal("expected insufficient finite request count to fail")
	}

	var stored model.Token
	if err := db.First(&stored, "id = ?", "token-1").Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if stored.RemainRequestCount != 1 {
		t.Fatalf("RemainRequestCount=%d, want unchanged 1", stored.RemainRequestCount)
	}
	if stored.UsedRequestCount != 2 {
		t.Fatalf("UsedRequestCount=%d, want unchanged 2", stored.UsedRequestCount)
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestGetAllAdminFilteredSpansUsers(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	seed := []*model.Token{
		{Id: "t1", UserId: "user-1", Key: "k1", Name: "alpha", Status: model.TokenStatusEnabled, CreatedTime: 100, UpdatedTime: 100},
		{Id: "t2", UserId: "user-2", Key: "k2", Name: "beta", Status: model.TokenStatusDisabled, CreatedTime: 200, UpdatedTime: 200},
		{Id: "t3", UserId: "user-2", Key: "k3", Name: "gamma", Status: model.TokenStatusEnabled, CreatedTime: 300, UpdatedTime: 300},
	}
	for _, tok := range seed {
		if err := db.Create(tok).Error; err != nil {
			t.Fatalf("create token %s: %v", tok.Id, err)
		}
	}

	// 无过滤:跨全部用户返回 3 条。
	all, err := GetAllAdminFiltered(0, 100, "", "", 0, "", "")
	if err != nil {
		t.Fatalf("GetAllAdminFiltered all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all tokens = %d, want 3", len(all))
	}
	total, err := CountAdminFiltered(0, "", "")
	if err != nil {
		t.Fatalf("CountAdminFiltered all: %v", err)
	}
	if total != 3 {
		t.Fatalf("count all = %d, want 3", total)
	}

	// 按属主过滤。
	byUser, err := GetAllAdminFiltered(0, 100, "", "", 0, "", "user-2")
	if err != nil {
		t.Fatalf("GetAllAdminFiltered user-2: %v", err)
	}
	if len(byUser) != 2 {
		t.Fatalf("user-2 tokens = %d, want 2", len(byUser))
	}

	// 按状态过滤。
	enabled, err := CountAdminFiltered(model.TokenStatusEnabled, "", "")
	if err != nil {
		t.Fatalf("CountAdminFiltered enabled: %v", err)
	}
	if enabled != 2 {
		t.Fatalf("enabled count = %d, want 2", enabled)
	}

	// 按关键词(name/id 模糊)过滤。
	byKeyword, err := GetAllAdminFiltered(0, 100, "", "", 0, "beta", "")
	if err != nil {
		t.Fatalf("GetAllAdminFiltered keyword: %v", err)
	}
	if len(byKeyword) != 1 || byKeyword[0].Id != "t2" {
		t.Fatalf("keyword tokens = %#v, want single t2", byKeyword)
	}
}
