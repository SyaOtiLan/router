package billing

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	channelcontroller "github.com/yeying-community/router/internal/admin/controller/channel"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

const (
	channelBillingSchedulerTickSeconds          = 30
	channelBillingAutoRefreshMinIntervalSeconds = 60
)

var startChannelBillingAutoRefreshWorkerOnce sync.Once

func StartChannelBillingAutoRefreshWorker() {
	startChannelBillingAutoRefreshWorkerOnce.Do(func() {
		go runChannelBillingAutoRefreshWorker()
	})
}

func runChannelBillingAutoRefreshWorker() {
	logger.SysLog("[billing.channel] auto refresh worker started")
	ticker := time.NewTicker(channelBillingSchedulerTickSeconds * time.Second)
	defer ticker.Stop()

	for {
		if shouldRunChannelBillingAutoRefreshNow() {
			runChannelBillingAutoRefreshOnce()
		}
		<-ticker.C
	}
}

func shouldRunChannelBillingAutoRefreshNow() bool {
	if !config.ChannelBillingAutoRefreshEnabled {
		return false
	}
	now := helper.GetTimestamp()
	intervalSeconds := int64(normalizedChannelBillingAutoRefreshIntervalSeconds())
	if config.ChannelBillingAutoRefreshLastRunAt <= 0 {
		return true
	}
	return now-config.ChannelBillingAutoRefreshLastRunAt >= intervalSeconds
}

func runChannelBillingAutoRefreshOnce() {
	runAt := helper.GetTimestamp()
	_ = model.UpdateOption("ChannelBillingAutoRefreshLastRunAt", fmt.Sprintf("%d", runAt))

	channels, err := channelsvc.GetAllBasic(0, 0, "all", true)
	if err != nil {
		logger.SysWarnf("[billing.channel] list channels failed: %s", err.Error())
		return
	}

	submittedCount := 0
	reusedCount := 0
	failedCount := 0
	for _, channel := range channels {
		if channel == nil || strings.TrimSpace(channel.Id) == "" {
			continue
		}
		if !shouldAutoRefreshChannelBilling(channel) {
			continue
		}
		task, reused, err := model.CreateOrReuseAsyncTaskWithDB(model.DB, model.AsyncTask{
			Type:      model.AsyncTaskTypeChannelRefreshBilling,
			DedupeKey: fmt.Sprintf("%s:%s", model.AsyncTaskTypeChannelRefreshBilling, strings.TrimSpace(channel.Id)),
			ChannelId: strings.TrimSpace(channel.Id),
			Payload: marshalChannelBillingSchedulerPayload(map[string]any{
				"channel_id": strings.TrimSpace(channel.Id),
			}),
			CreatedBy: "",
			TraceID:   "",
		})
		if err != nil {
			failedCount++
			logger.SysWarnf("[billing.channel] enqueue refresh failed channel_id=%s err=%s", strings.TrimSpace(channel.Id), err.Error())
			continue
		}
		if reused {
			reusedCount++
			continue
		}
		if strings.TrimSpace(task.Id) != "" {
			submittedCount++
		}
	}
	runChannelProviderUsageSyncScheduling(channels)

	logger.SysLogf("[billing.channel] auto refresh queued submitted=%d reused=%d failed=%d", submittedCount, reusedCount, failedCount)
}

func runChannelProviderUsageSyncScheduling(channels []*model.Channel) {
	submitted, reused, failed := 0, 0, 0
	for _, channel := range channels {
		if !shouldAutoRefreshChannelBilling(channel) || strings.TrimSpace(channel.Id) == "" {
			continue
		}
		profile, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channel.Id)
		if err != nil || !supportsProviderUsageSync(profile.BillingSource) {
			continue
		}
		_, wasReused, err := channelcontroller.CreateChannelProviderUsageSyncTask(channel.Id, "billing_scheduler", "")
		if err != nil {
			failed++
			logger.SysWarnf("[billing.channel] enqueue usage sync failed channel_id=%s err=%s", channel.Id, err.Error())
			continue
		}
		if wasReused {
			reused++
		} else {
			submitted++
		}
	}
	if submitted > 0 || reused > 0 || failed > 0 {
		logger.SysLogf("[billing.channel] provider usage sync queued submitted=%d reused=%d failed=%d", submitted, reused, failed)
	}
}

func supportsProviderUsageSync(source string) bool {
	normalized := strings.ToLower(strings.TrimSpace(source))
	if normalized == "" || normalized == model.ChannelBillingSourceManual || normalized == "unsupported" || strings.HasPrefix(normalized, "builtin_") {
		return false
	}
	return true
}

func marshalChannelBillingSchedulerPayload(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(body)
}

func normalizedChannelBillingAutoRefreshIntervalSeconds() int {
	interval := config.ChannelBillingAutoRefreshIntervalSeconds
	if interval < channelBillingAutoRefreshMinIntervalSeconds {
		interval = channelBillingAutoRefreshMinIntervalSeconds
	}
	return interval
}

func shouldAutoRefreshChannelBilling(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	return channel.Status == model.ChannelStatusEnabled
}
