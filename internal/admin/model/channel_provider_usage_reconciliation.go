package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ListChannelConsumeLogsForUsageReconciliationWithDB returns the immutable
// request facts needed by the provider-ledger matcher. Finance records are
// hydrated when their normalized tables are present, but reconciliation never
// writes or changes those records.
func ListChannelConsumeLogsForUsageReconciliationWithDB(db *gorm.DB, channelID string, startAt, endAt int64, modelName string, limit int) ([]Log, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}
	query := db.Where("type = ? AND channel_id = ?", LogTypeConsume, channelID)
	if startAt > 0 {
		query = query.Where("created_at >= ?", startAt)
	}
	if endAt > 0 {
		query = query.Where("created_at <= ?", endAt)
	}
	if modelName = strings.TrimSpace(modelName); modelName != "" {
		query = query.Where("(model_name = ? OR request_model_name = ? OR actual_model_name = ?)", modelName, modelName, modelName)
	}
	rows := make([]Log, 0)
	if err := query.Order("created_at ASC, id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := hydrateChannelUsageReconciliationFinanceWithDB(db, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func hydrateChannelUsageReconciliationFinanceWithDB(db *gorm.DB, rows []Log) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]string, 0, len(rows))
	byID := make(map[string]*Log, len(rows))
	for index := range rows {
		id := strings.TrimSpace(rows[index].Id)
		if id == "" {
			continue
		}
		ids = append(ids, id)
		byID[id] = &rows[index]
	}
	if len(ids) == 0 {
		return nil
	}
	if db.Migrator().HasTable(&BillingSettlement{}) {
		settlements := make([]BillingSettlement, 0, len(ids))
		if err := db.Where("request_log_id IN ?", ids).Find(&settlements).Error; err != nil {
			return err
		}
		for _, settlement := range settlements {
			row := byID[settlement.RequestLogID]
			if row == nil {
				continue
			}
			row.BillingCurrency = settlement.Currency
			row.BillingOfficialAnchorAmount = settlement.OfficialAnchorAmount
			row.BillingOfficialAnchorCurrency = settlement.OfficialAnchorCurrency
			row.BillingOfficialAnchorBaseAmount = settlement.OfficialAnchorBaseAmount
			row.PromptTokens = settlement.PromptTokens
			row.CompletionTokens = settlement.CompletionTokens
			row.BillingPromptTokenDelta = settlement.PromptTokenDelta
			row.BillingOutputTokenDelta = settlement.OutputTokenDelta
		}
	}
	if db.Migrator().HasTable(&ProcurementAttribution{}) {
		attributions := make([]ProcurementAttribution, 0, len(ids))
		if err := db.Where("request_log_id IN ?", ids).Find(&attributions).Error; err != nil {
			return err
		}
		for _, attribution := range attributions {
			row := byID[attribution.RequestLogID]
			if row == nil {
				continue
			}
			row.BillingProcurementCostBaseAmount = attribution.CostBaseAmount
			row.BillingProcurementCostSource = attribution.CostSource
			row.BillingProcurementCostStatus = attribution.Status
		}
	}
	return nil
}
