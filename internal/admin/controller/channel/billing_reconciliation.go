package channel

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

const (
	channelUsageReconciliationMatched        = "matched"
	channelUsageReconciliationRouterOnly     = "router_only"
	channelUsageReconciliationProviderOnly   = "provider_only"
	channelUsageReconciliationUsageMismatch  = "usage_mismatch"
	channelUsageReconciliationAmountMismatch = "amount_mismatch"
	channelUsageReconciliationPending        = "pending"
	channelUsageReconciliationPendingWindow  = int64(5 * 60)
	channelUsageReconciliationRouterCurrency = "CNY"
)

type channelUsageReconciliationItem struct {
	Status               string  `json:"status"`
	Reason               string  `json:"reason,omitempty"`
	ProviderRecordID     string  `json:"provider_record_id,omitempty"`
	RouterRequestLogID   string  `json:"router_request_log_id,omitempty"`
	ProviderOccurredAt   int64   `json:"provider_occurred_at,omitempty"`
	RouterCreatedAt      int64   `json:"router_created_at,omitempty"`
	Model                string  `json:"model,omitempty"`
	ProviderInputTokens  int64   `json:"provider_input_tokens,omitempty"`
	RouterInputTokens    int64   `json:"router_input_tokens,omitempty"`
	InputTokenDelta      int64   `json:"input_token_delta,omitempty"`
	ProviderOutputTokens int64   `json:"provider_output_tokens,omitempty"`
	RouterOutputTokens   int64   `json:"router_output_tokens,omitempty"`
	OutputTokenDelta     int64   `json:"output_token_delta,omitempty"`
	ProviderCostAmount   float64 `json:"provider_cost_amount,omitempty"`
	ProviderCurrency     string  `json:"provider_currency,omitempty"`
	RouterCostBaseAmount float64 `json:"router_cost_base_amount,omitempty"`
	AmountComparable     bool    `json:"amount_comparable"`
	AmountDelta          float64 `json:"amount_delta,omitempty"`
}

type channelUsageReconciliationSummary struct {
	ProviderRecordCount int `json:"provider_record_count"`
	RouterRequestCount  int `json:"router_request_count"`
	PairedCount         int `json:"paired_count"`
	MatchedCount        int `json:"matched_count"`
	UsageMismatchCount  int `json:"usage_mismatch_count"`
	AmountMismatchCount int `json:"amount_mismatch_count"`
	RouterOnlyCount     int `json:"router_only_count"`
	ProviderOnlyCount   int `json:"provider_only_count"`
	PendingCount        int `json:"pending_count"`
}

type channelUsageReconciliationData struct {
	ChannelID     string                            `json:"channel_id"`
	Adapter       string                            `json:"adapter,omitempty"`
	WindowStartAt int64                             `json:"window_start_at"`
	WindowEndAt   int64                             `json:"window_end_at"`
	Summary       channelUsageReconciliationSummary `json:"summary"`
	Items         []channelUsageReconciliationItem  `json:"items"`
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func normalizedUsageModel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if index := strings.LastIndex(value, "/"); index >= 0 {
		value = value[index+1:]
	}
	return value
}

func usageModelsMatch(providerModel string, routerLog model.Log) bool {
	providerModel = normalizedUsageModel(providerModel)
	if providerModel == "" {
		return false
	}
	for _, candidate := range []string{routerLog.ActualModelName, routerLog.RequestModelName, routerLog.ModelName} {
		if providerModel == normalizedUsageModel(candidate) {
			return true
		}
	}
	return false
}

func providerUsageMatchWindowSeconds(record model.ChannelProviderUsageRecord) int64 {
	window := int64(30)
	if record.DurationMs > 0 {
		window = record.DurationMs/1000 + 15
		if window < 30 {
			window = 30
		}
	}
	if window > 300 {
		window = 300
	}
	return window
}

type usageMatchCandidate struct {
	Index      int
	TimeDelta  int64
	TokenDelta int64
}

func findProviderUsageLogMatch(record model.ChannelProviderUsageRecord, logs []model.Log, used map[int]bool) (int, bool) {
	if strings.TrimSpace(record.Model) == "" || record.OccurredAt <= 0 {
		return -1, false
	}
	window := providerUsageMatchWindowSeconds(record)
	candidates := make([]usageMatchCandidate, 0)
	for index, row := range logs {
		if used[index] || !usageModelsMatch(record.Model, row) {
			continue
		}
		timeDelta := absInt64(record.OccurredAt - row.CreatedAt)
		if timeDelta > window {
			continue
		}
		inputDelta := absInt64(record.InputTokens - int64(row.PromptTokens))
		outputDelta := absInt64(record.OutputTokens - int64(row.CompletionTokens))
		candidates = append(candidates, usageMatchCandidate{Index: index, TimeDelta: timeDelta, TokenDelta: inputDelta + outputDelta})
	}
	if len(candidates) == 0 {
		return -1, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].TokenDelta != candidates[j].TokenDelta {
			return candidates[i].TokenDelta < candidates[j].TokenDelta
		}
		if candidates[i].TimeDelta != candidates[j].TimeDelta {
			return candidates[i].TimeDelta < candidates[j].TimeDelta
		}
		return candidates[i].Index < candidates[j].Index
	})
	return candidates[0].Index, true
}

func buildUsageReconciliationItem(record model.ChannelProviderUsageRecord, log *model.Log, status, reason string) channelUsageReconciliationItem {
	item := channelUsageReconciliationItem{
		Status:               status,
		Reason:               reason,
		ProviderRecordID:     record.UpstreamRecordId,
		ProviderOccurredAt:   record.OccurredAt,
		Model:                record.Model,
		ProviderInputTokens:  record.InputTokens,
		ProviderOutputTokens: record.OutputTokens,
		ProviderCostAmount:   record.CostAmount,
		ProviderCurrency:     strings.ToUpper(strings.TrimSpace(record.Currency)),
	}
	if log == nil {
		return item
	}
	item.RouterRequestLogID = log.Id
	item.RouterCreatedAt = log.CreatedAt
	if item.Model == "" {
		item.Model = strings.TrimSpace(log.ActualModelName)
	}
	item.RouterInputTokens = int64(log.PromptTokens)
	item.RouterOutputTokens = int64(log.CompletionTokens)
	item.InputTokenDelta = record.InputTokens - item.RouterInputTokens
	item.OutputTokenDelta = record.OutputTokens - item.RouterOutputTokens
	item.RouterCostBaseAmount = log.BillingProcurementCostBaseAmount
	providerCurrency := strings.ToUpper(strings.TrimSpace(record.Currency))
	item.AmountComparable = providerCurrency != "" && providerCurrency == channelUsageReconciliationRouterCurrency && log.BillingProcurementCostStatus != ""
	if item.AmountComparable {
		item.AmountDelta = record.CostAmount - item.RouterCostBaseAmount
	}
	return item
}

func reconcileChannelProviderUsage(records []model.ChannelProviderUsageRecord, logs []model.Log, now int64) (channelUsageReconciliationSummary, []channelUsageReconciliationItem) {
	summary := channelUsageReconciliationSummary{ProviderRecordCount: len(records), RouterRequestCount: len(logs)}
	items := make([]channelUsageReconciliationItem, 0, len(records)+len(logs))
	usedLogs := make(map[int]bool, len(logs))
	for _, record := range records {
		index, matched := findProviderUsageLogMatch(record, logs, usedLogs)
		if !matched {
			status := channelUsageReconciliationProviderOnly
			if record.OccurredAt >= now-channelUsageReconciliationPendingWindow {
				status = channelUsageReconciliationPending
				summary.PendingCount++
			} else {
				summary.ProviderOnlyCount++
			}
			items = append(items, buildUsageReconciliationItem(record, nil, status, "no Router request matched within the time and model window"))
			continue
		}
		usedLogs[index] = true
		summary.PairedCount++
		item := buildUsageReconciliationItem(record, &logs[index], channelUsageReconciliationMatched, "")
		usageMismatch := item.InputTokenDelta != 0 || item.OutputTokenDelta != 0
		amountMismatch := item.AmountComparable && math.Abs(item.AmountDelta) > math.Max(0.01, math.Abs(record.CostAmount)*0.01)
		if usageMismatch {
			item.Status = channelUsageReconciliationUsageMismatch
			item.Reason = "provider and Router input/output token counts differ"
			summary.UsageMismatchCount++
		} else if amountMismatch {
			item.Status = channelUsageReconciliationAmountMismatch
			item.Reason = "provider cost differs from Router procurement cost"
			summary.AmountMismatchCount++
		} else {
			summary.MatchedCount++
		}
		items = append(items, item)
	}
	for index := range logs {
		if usedLogs[index] {
			continue
		}
		status := channelUsageReconciliationRouterOnly
		if logs[index].CreatedAt >= now-channelUsageReconciliationPendingWindow {
			status = channelUsageReconciliationPending
			summary.PendingCount++
		} else {
			summary.RouterOnlyCount++
		}
		items = append(items, buildUsageReconciliationItem(model.ChannelProviderUsageRecord{}, &logs[index], status, "no provider usage record matched within the time and model window"))
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].ProviderOccurredAt
		if left == 0 {
			left = items[i].RouterCreatedAt
		}
		right := items[j].ProviderOccurredAt
		if right == 0 {
			right = items[j].RouterCreatedAt
		}
		if left != right {
			return left < right
		}
		return items[i].ProviderRecordID < items[j].ProviderRecordID
	})
	return summary, items
}

func parseReconciliationUnixQuery(c *gin.Context, name string, defaultValue int64) (int64, error) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, strconv.ErrSyntax
	}
	return parsed, nil
}

func GetChannelProviderUsageReconciliation(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if _, err := channelsvc.GetByID(channelID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "渠道不存在"})
		return
	}
	now := time.Now().Unix()
	endAt, err := parseReconciliationUnixQuery(c, "end_at", now)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "end_at 必须是 Unix 时间戳"})
		return
	}
	startAt, err := parseReconciliationUnixQuery(c, "start_at", endAt-24*60*60)
	if err != nil || startAt > endAt {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "start_at 必须不晚于 end_at"})
		return
	}
	limit, err := parseReconciliationUnixQuery(c, "limit", 1000)
	if err != nil || limit <= 0 || limit > 5000 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "limit 必须在 1 到 5000 之间"})
		return
	}
	adapter := strings.ToLower(strings.TrimSpace(c.Query("adapter")))
	if adapter == "" {
		if profile, profileErr := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channelID); profileErr == nil {
			adapter = resolveBillingServiceAdapter(profile)
		}
	}
	modelName := strings.TrimSpace(c.Query("model"))
	statusCode := 0
	if rawStatus := strings.TrimSpace(c.Query("status_code")); rawStatus != "" {
		statusCode, err = strconv.Atoi(rawStatus)
		if err != nil || statusCode < 100 || statusCode > 599 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "status_code 必须是有效 HTTP 状态码"})
			return
		}
	}
	providerRows, err := model.ListChannelProviderUsageRecordsForWindowAndStatusWithDB(model.DB, channelID, adapter, modelName, startAt, endAt, statusCode, int(limit))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	logDB := model.LOG_DB
	if logDB == nil {
		logDB = model.DB
	}
	logs, err := model.ListChannelConsumeLogsForUsageReconciliationWithDB(logDB, channelID, startAt, endAt, modelName, int(limit))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	summary, items := reconcileChannelProviderUsage(providerRows, logs, now)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": channelUsageReconciliationData{ChannelID: channelID, Adapter: adapter, WindowStartAt: startAt, WindowEndAt: endAt, Summary: summary, Items: items}})
}
