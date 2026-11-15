package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const TokenGroupRoutesTableName = "api_token_group_routes"

// TokenGroupRoute stores a user-managed preference order. It does not grant
// access to a group and is not consumed by relay routing in phase one.
type TokenGroupRoute struct {
	TokenID    string `json:"token_id" gorm:"type:char(36);primaryKey;uniqueIndex:idx_token_group_route_order,priority:1"`
	GroupID    string `json:"group_id" gorm:"type:char(36);primaryKey"`
	OrderIndex int    `json:"order_index" gorm:"not null;uniqueIndex:idx_token_group_route_order,priority:2"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;not null;default:0"`
	UpdatedAt  int64  `json:"updated_at" gorm:"bigint;not null;default:0"`
}

func (TokenGroupRoute) TableName() string { return TokenGroupRoutesTableName }

func ListTokenGroupRouteOrderWithDB(db *gorm.DB, tokenID string) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedTokenID := strings.TrimSpace(tokenID)
	if normalizedTokenID == "" {
		return []string{}, nil
	}
	rows := make([]TokenGroupRoute, 0)
	if err := db.Where("token_id = ?", normalizedTokenID).Order("order_index asc, group_id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, strings.TrimSpace(row.GroupID))
	}
	return result, nil
}

func ListTokenGroupRouteOrder(tokenID string) ([]string, error) {
	return ListTokenGroupRouteOrderWithDB(DB, tokenID)
}

func ReplaceTokenGroupRouteOrderWithDB(db *gorm.DB, tokenID string, groupIDs []string, now int64) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedTokenID := strings.TrimSpace(tokenID)
	if normalizedTokenID == "" {
		return fmt.Errorf("token ID is required")
	}
	if now <= 0 {
		now = helper.GetTimestamp()
	}
	rows := make([]TokenGroupRoute, 0, len(groupIDs))
	seen := make(map[string]struct{}, len(groupIDs))
	for index, groupID := range groupIDs {
		normalizedGroupID := strings.TrimSpace(groupID)
		if normalizedGroupID == "" {
			return fmt.Errorf("group ID is required")
		}
		if _, exists := seen[normalizedGroupID]; exists {
			return fmt.Errorf("duplicate group ID: %s", normalizedGroupID)
		}
		seen[normalizedGroupID] = struct{}{}
		rows = append(rows, TokenGroupRoute{TokenID: normalizedTokenID, GroupID: normalizedGroupID, OrderIndex: index, CreatedAt: now, UpdatedAt: now})
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("token_id = ?", normalizedTokenID).Delete(&TokenGroupRoute{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func ReplaceTokenGroupRouteOrder(tokenID string, groupIDs []string) error {
	return ReplaceTokenGroupRouteOrderWithDB(DB, tokenID, groupIDs, helper.GetTimestamp())
}
