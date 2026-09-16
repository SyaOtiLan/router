package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	"gorm.io/gorm"
)

type billingServiceUsageRequest struct {
	ChannelID   string            `json:"channel_id,omitempty"`
	Adapter     string            `json:"adapter"`
	Credentials map[string]string `json:"credentials,omitempty"`
	Cursor      string            `json:"cursor,omitempty"`
	Limit       int               `json:"limit,omitempty"`
	Model       string            `json:"model,omitempty"`
}
type billingServiceUsageRecord struct {
	RecordID         string         `json:"record_id"`
	OccurredAt       *time.Time     `json:"occurred_at,omitempty"`
	Model            string         `json:"model,omitempty"`
	InputTokens      int64          `json:"input_tokens,omitempty"`
	OutputTokens     int64          `json:"output_tokens,omitempty"`
	CacheReadTokens  int64          `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int64          `json:"cache_write_tokens,omitempty"`
	Cost             float64        `json:"cost,omitempty"`
	Currency         string         `json:"currency,omitempty"`
	StatusCode       int            `json:"status_code,omitempty"`
	DurationMS       int64          `json:"duration_ms,omitempty"`
	TTFBMS           int64          `json:"ttfb_ms,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}
type billingServiceUsagePage struct {
	ChannelID  string                      `json:"channel_id,omitempty"`
	Adapter    string                      `json:"adapter"`
	Records    []billingServiceUsageRecord `json:"records"`
	NextCursor string                      `json:"next_cursor,omitempty"`
	HasMore    bool                        `json:"has_more"`
	FetchedAt  time.Time                   `json:"fetched_at"`
}
type billingServiceUsageResponse struct {
	Data  billingServiceUsagePage `json:"data"`
	Error *billingServiceError    `json:"error,omitempty"`
}

func fetchBillingUsage(ctx context.Context, req billingServiceUsageRequest) (billingServiceUsagePage, error) {
	base := strings.TrimRight(strings.TrimSpace(config.BillingServiceBaseURL), "/")
	if base == "" {
		return billingServiceUsagePage{}, fmt.Errorf("Billing 服务未配置")
	}
	body, _ := json.Marshal(req)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/internal/billing:usage", strings.NewReader(string(body)))
	if err != nil {
		return billingServiceUsagePage{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if key := strings.TrimSpace(config.BillingServiceAPIKey); key != "" {
		request.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return billingServiceUsagePage{}, err
	}
	defer resp.Body.Close()
	decoded := billingServiceUsageResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return billingServiceUsagePage{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decoded.Error != nil {
			return billingServiceUsagePage{}, fmt.Errorf("Billing 服务返回 %s: %s", decoded.Error.Code, decoded.Error.Message)
		}
		return billingServiceUsagePage{}, fmt.Errorf("Billing 服务返回 HTTP %d", resp.StatusCode)
	}
	return decoded.Data, nil
}

func recordChannelProviderUsageSyncFailure(channelID, adapter string, state model.ChannelProviderUsageSyncState, err error) {
	now := time.Now().Unix()
	state.ChannelId = channelID
	state.Adapter = adapter
	state.LastError = strings.TrimSpace(err.Error())
	state.ConsecutiveFailures++
	state.UpdatedAt = now
	_ = model.SaveChannelProviderUsageSyncStateWithDB(model.DB, state)
}

func SyncChannelProviderUsage(c *gin.Context) {
	channel, err := channelsvc.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "渠道不存在"})
		return
	}
	profile, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channel.Id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	adapter := resolveBillingServiceAdapter(profile)
	if adapter == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "当前渠道未配置 Billing adapter"})
		return
	}
	query, err := buildBillingServiceQuery(c.Request.Context(), channel, profile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	state, _ := model.GetChannelProviderUsageSyncStateWithDB(model.DB, channel.Id, adapter)
	page, err := fetchBillingUsage(c, billingServiceUsageRequest{ChannelID: query.ChannelID, Adapter: query.Adapter, Credentials: query.Credentials, Cursor: state.Cursor, Limit: 100})
	if err != nil {
		recordChannelProviderUsageSyncFailure(channel.Id, adapter, state, err)
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	now := time.Now().Unix()
	count := 0
	for _, item := range page.Records {
		occurred := now
		if item.OccurredAt != nil {
			occurred = item.OccurredAt.Unix()
		}
		metadata, _ := json.Marshal(item.Metadata)
		_, err = model.UpsertChannelProviderUsageRecordWithDB(model.DB, model.ChannelProviderUsageRecord{ChannelId: channel.Id, Adapter: adapter, UpstreamRecordId: item.RecordID, OccurredAt: occurred, Model: item.Model, InputTokens: item.InputTokens, OutputTokens: item.OutputTokens, CacheReadTokens: item.CacheReadTokens, CacheWriteTokens: item.CacheWriteTokens, CostAmount: item.Cost, Currency: item.Currency, StatusCode: item.StatusCode, DurationMs: item.DurationMS, TTFBMs: item.TTFBMS, Metadata: string(metadata), FetchedAt: now})
		if err != nil {
			recordChannelProviderUsageSyncFailure(channel.Id, adapter, state, err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			return
		}
		count++
	}
	state.ChannelId, state.Adapter, state.Cursor, state.LastSuccessAt, state.LastError, state.ConsecutiveFailures, state.UpdatedAt = channel.Id, adapter, page.NextCursor, now, "", 0, now
	_ = model.SaveChannelProviderUsageSyncStateWithDB(model.DB, state)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"fetched": count, "has_more": page.HasMore, "next_cursor": page.NextCursor}})
}

func GetChannelProviderUsage(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rows, err := model.ListChannelProviderUsageRecordsWithDB(model.DB, c.Param("id"), strings.TrimSpace(c.Query("adapter")), strings.TrimSpace(c.Query("model")), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

// GetChannelProviderUsageSyncState exposes operational health without
// returning the opaque provider cursor or stored credentials.
func GetChannelProviderUsageSyncState(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	adapter := strings.ToLower(strings.TrimSpace(c.Query("adapter")))
	if adapter == "" {
		if profile, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channelID); err == nil {
			adapter = resolveBillingServiceAdapter(profile)
		}
	}
	if adapter == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "adapter 未配置"})
		return
	}
	state, err := model.GetChannelProviderUsageSyncStateWithDB(model.DB, channelID, adapter)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"channel_id": channelID, "adapter": adapter, "cursor_present": false, "last_success_at": 0, "last_error": "", "consecutive_failures": 0, "updated_at": 0}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"channel_id": state.ChannelId, "adapter": state.Adapter, "cursor_present": strings.TrimSpace(state.Cursor) != "", "last_success_at": state.LastSuccessAt, "last_error": state.LastError, "consecutive_failures": state.ConsecutiveFailures, "updated_at": state.UpdatedAt}})
}
