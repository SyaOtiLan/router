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

type channelProviderUsageSyncError struct {
	status int
	err    error
}

func (e *channelProviderUsageSyncError) Error() string {
	if e == nil || e.err == nil {
		return "渠道 usage 同步失败"
	}
	return e.err.Error()
}

func (e *channelProviderUsageSyncError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func channelProviderUsageError(status int, err error) error {
	if err == nil {
		return nil
	}
	return &channelProviderUsageSyncError{status: status, err: err}
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

func syncChannelProviderUsage(ctx context.Context, channelID string) (map[string]any, error) {
	channel, err := channelsvc.GetByID(channelID)
	if err != nil {
		return nil, channelProviderUsageError(http.StatusNotFound, err)
	}
	profile, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channel.Id)
	if err != nil {
		return nil, channelProviderUsageError(http.StatusBadRequest, err)
	}
	adapter := resolveBillingServiceAdapter(profile)
	if adapter == "" {
		return nil, channelProviderUsageError(http.StatusBadRequest, fmt.Errorf("当前渠道未配置 Billing adapter"))
	}
	query, err := buildBillingServiceQuery(ctx, channel, profile)
	if err != nil {
		return nil, channelProviderUsageError(http.StatusBadRequest, err)
	}
	state, _ := model.GetChannelProviderUsageSyncStateWithDB(model.DB, channel.Id, adapter)
	page, err := fetchBillingUsage(ctx, billingServiceUsageRequest{ChannelID: query.ChannelID, Adapter: query.Adapter, Credentials: query.Credentials, Cursor: state.Cursor, Limit: 100})
	if err != nil {
		recordChannelProviderUsageSyncFailure(channel.Id, adapter, state, err)
		return nil, channelProviderUsageError(http.StatusBadGateway, err)
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
			return nil, channelProviderUsageError(http.StatusInternalServerError, err)
		}
		count++
	}
	state.ChannelId, state.Adapter, state.Cursor, state.LastSuccessAt, state.LastError, state.ConsecutiveFailures, state.UpdatedAt = channel.Id, adapter, page.NextCursor, now, "", 0, now
	if err := model.SaveChannelProviderUsageSyncStateWithDB(model.DB, state); err != nil {
		return nil, channelProviderUsageError(http.StatusInternalServerError, err)
	}
	return map[string]any{"channel_id": channel.Id, "adapter": adapter, "fetched": count, "has_more": page.HasMore, "next_cursor": page.NextCursor}, nil
}

func SyncChannelProviderUsageTask(ctx context.Context, channelID string) (string, error) {
	result, err := syncChannelProviderUsage(ctx, strings.TrimSpace(channelID))
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func SyncChannelProviderUsage(c *gin.Context) {
	result, err := syncChannelProviderUsage(c.Request.Context(), c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		var syncErr *channelProviderUsageSyncError
		if errors.As(err, &syncErr) && syncErr.status > 0 {
			status = syncErr.status
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
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
