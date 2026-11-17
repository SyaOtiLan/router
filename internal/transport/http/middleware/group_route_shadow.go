package middleware

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/routing"
)

const (
	groupRouteShadowQueueCapacity = 128
	groupRouteShadowWorkerCount   = 2
)

type groupRouteShadowTask struct {
	TraceID             string
	TokenID             string
	UserID              string
	RequestModel        string
	RequestPath         string
	ActualGroupID       string
	ActualChannelID     string
	ActualSource        string
	PersonalRoutePolicy string
	ProviderPolicy      routing.ProviderRoutingPolicy
	EnqueuedAt          time.Time
}

var (
	groupRouteShadowStartOnce sync.Once
	groupRouteShadowQueue     chan groupRouteShadowTask
	groupRouteShadowDrops     atomic.Uint64
)

func groupRouteShadowEnabledForToken(tokenID string) bool {
	normalizedTokenID := strings.TrimSpace(tokenID)
	if !config.GroupRouteShadowEnabled || normalizedTokenID == "" || config.GroupRouteShadowSampleRate <= 0 {
		return false
	}
	allowed := false
	for _, allowedTokenID := range config.GroupRouteShadowTokenAllowlist {
		if normalizedTokenID == strings.TrimSpace(allowedTokenID) {
			allowed = true
			break
		}
	}
	if !allowed {
		return false
	}
	if config.GroupRouteShadowSampleRate >= 1 {
		return true
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(normalizedTokenID))
	sample := float64(hash.Sum32()) / (float64(math.MaxUint32) + 1)
	return sample < config.GroupRouteShadowSampleRate
}

func logGroupRouteShadowSkipped(c *gin.Context, tokenID string, reason string) {
	if c == nil || c.Request == nil {
		return
	}
	logger.RelayInfof(c.Request.Context(), "ROUTE_SHADOW outcome=skipped reason=%s trace_id=%s token_id=%s", reason, helper.GetTraceID(c.Request.Context()), strings.TrimSpace(tokenID))
}

func submitGroupRouteShadow(c *gin.Context, actualGroupID string, channel *model.ChannelCandidate) {
	if c == nil || channel == nil {
		return
	}
	tokenID := c.GetString(ctxkey.TokenId)
	if !groupRouteShadowEnabledForToken(tokenID) {
		return
	}
	if _, explicitChannel := c.Get(ctxkey.SpecificChannelId); explicitChannel {
		logGroupRouteShadowSkipped(c, tokenID, "explicit_channel")
		return
	}
	if c.GetBool(ctxkey.ResponsesStatefulRequest) {
		logGroupRouteShadowSkipped(c, tokenID, "stateful_responses_request")
		return
	}
	if c.GetBool(ctxkey.ResponsesStateReset) {
		logGroupRouteShadowSkipped(c, tokenID, "responses_state_reset")
		return
	}
	if strings.TrimSpace(c.GetString(ctxkey.CommunityOfferID)) != "" {
		logGroupRouteShadowSkipped(c, tokenID, "community_offer")
		return
	}
	if requestBodyIsForm(c) {
		logGroupRouteShadowSkipped(c, tokenID, "form_request")
		return
	}
	if model.NormalizeRequestedChannelModelEndpoint(c.Request.URL.Path) == "" {
		logGroupRouteShadowSkipped(c, tokenID, "unsupported_endpoint")
		return
	}
	policy, ok := c.Get(ctxkey.ProviderRoutingPolicy)
	if !ok {
		logGroupRouteShadowSkipped(c, tokenID, "provider_policy_missing")
		return
	}
	providerPolicy, ok := policy.(routing.ProviderRoutingPolicy)
	if !ok {
		logGroupRouteShadowSkipped(c, tokenID, "provider_policy_invalid")
		return
	}
	task := groupRouteShadowTask{
		TraceID:             helper.GetTraceID(c.Request.Context()),
		TokenID:             strings.TrimSpace(tokenID),
		UserID:              c.GetString(ctxkey.Id),
		RequestModel:        strings.TrimSpace(c.GetString(ctxkey.RequestModel)),
		RequestPath:         c.Request.URL.Path,
		ActualGroupID:       strings.TrimSpace(actualGroupID),
		ActualChannelID:     strings.TrimSpace(channel.Id),
		ActualSource:        strings.TrimSpace(channel.SourceType),
		PersonalRoutePolicy: strings.TrimSpace(c.GetString(ctxkey.PersonalRoutePolicy)),
		ProviderPolicy:      providerPolicy,
		EnqueuedAt:          time.Now(),
	}
	if task.UserID == "" || task.RequestModel == "" {
		logGroupRouteShadowSkipped(c, tokenID, "request_context_incomplete")
		return
	}
	startGroupRouteShadowWorkers()
	select {
	case groupRouteShadowQueue <- task:
	default:
		dropped := groupRouteShadowDrops.Add(1)
		if dropped == 1 || dropped%100 == 0 {
			logger.RelayWarnf(c.Request.Context(), "ROUTE_SHADOW outcome=dropped reason=queue_full total_dropped=%d", dropped)
		}
	}
}

func startGroupRouteShadowWorkers() {
	groupRouteShadowStartOnce.Do(func() {
		groupRouteShadowQueue = make(chan groupRouteShadowTask, groupRouteShadowQueueCapacity)
		for worker := 0; worker < groupRouteShadowWorkerCount; worker++ {
			go runGroupRouteShadowWorker(groupRouteShadowQueue)
		}
	})
}

func runGroupRouteShadowWorker(queue <-chan groupRouteShadowTask) {
	for task := range queue {
		if time.Since(task.EnqueuedAt) > 5*time.Second {
			logger.RelayWarnf(context.Background(), "ROUTE_SHADOW outcome=dropped reason=stale_task trace_id=%s", task.TraceID)
			continue
		}
		evaluateGroupRouteShadow(task)
	}
}

func evaluateGroupRouteShadow(task groupRouteShadowTask) {
	startedAt := time.Now()
	timeout := config.GroupRouteShadowTimeout
	if timeout <= 0 {
		timeout = 250 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	log := func(format string, args ...any) {
		logger.RelayInfof(ctx, format, args...)
	}
	logSkip := func(reason string) {
		log("ROUTE_SHADOW outcome=skipped reason=%s trace_id=%s token_id=%s elapsed_ms=%d", reason, task.TraceID, task.TokenID, time.Since(startedAt).Milliseconds())
	}
	if task.PersonalRoutePolicy == model.PersonalRoutePolicyPersonalOnly {
		log("ROUTE_SHADOW outcome=evaluated comparison=incomparable reason=legacy_personal_only trace_id=%s token_id=%s actual_group_id=%s actual_source=%s elapsed_ms=%d", task.TraceID, task.TokenID, task.ActualGroupID, task.ActualSource, time.Since(startedAt).Milliseconds())
		return
	}
	if model.DB == nil {
		logSkip("database_unavailable")
		return
	}
	readDB := model.DB.WithContext(ctx)
	payload, err := model.BuildUserEntitlementModelsReadOnlyWithDB(ctx, readDB, task.UserID)
	if err != nil {
		log("ROUTE_SHADOW outcome=failed reason=entitlement_lookup trace_id=%s token_id=%s elapsed_ms=%d error=%q", task.TraceID, task.TokenID, time.Since(startedAt).Milliseconds(), err.Error())
		return
	}
	if ctx.Err() != nil {
		logSkip("evaluation_timeout")
		return
	}
	sources := payload.ByModel[task.RequestModel]
	if len(sources) == 0 {
		log("ROUTE_SHADOW outcome=evaluated comparison=no_entitlement trace_id=%s token_id=%s actual_group_id=%s actual_source=%s elapsed_ms=%d", task.TraceID, task.TokenID, task.ActualGroupID, task.ActualSource, time.Since(startedAt).Milliseconds())
		return
	}
	storedOrder, err := model.ListTokenGroupRouteOrderWithDB(readDB, task.TokenID)
	if err != nil {
		log("ROUTE_SHADOW outcome=failed reason=token_group_order_lookup trace_id=%s token_id=%s elapsed_ms=%d error=%q", task.TraceID, task.TokenID, time.Since(startedAt).Milliseconds(), err.Error())
		return
	}
	if ctx.Err() != nil {
		logSkip("evaluation_timeout")
		return
	}
	orderedGroups := orderEntitledGroupsByPreference(storedOrder, sources)
	if len(orderedGroups) == 0 {
		log("ROUTE_SHADOW outcome=evaluated comparison=no_entitlement trace_id=%s token_id=%s actual_group_id=%s actual_source=%s elapsed_ms=%d", task.TraceID, task.TokenID, task.ActualGroupID, task.ActualSource, time.Since(startedAt).Milliseconds())
		return
	}
	groupCounts := make([]string, 0, len(orderedGroups))
	shadowGroupID := ""
	endpointFiltered := 0
	providerFiltered := 0
	for _, groupID := range orderedGroups {
		if ctx.Err() != nil {
			logSkip("evaluation_timeout")
			return
		}
		channels, stats, listErr := model.CacheListPlatformSatisfiedChannelsForRequestWithStats(ctx, groupID, task.RequestModel, task.RequestPath)
		if listErr != nil {
			log("ROUTE_SHADOW outcome=failed reason=candidate_lookup trace_id=%s token_id=%s group_id=%s elapsed_ms=%d error=%q", task.TraceID, task.TokenID, groupID, time.Since(startedAt).Milliseconds(), listErr.Error())
			return
		}
		endpointFiltered += len(stats.FilteredCandidates)
		channels, filtered := applyProviderRoutingPolicy(channels, task.RequestModel, task.ProviderPolicy)
		providerFiltered += len(filtered)
		groupCounts = append(groupCounts, fmt.Sprintf("%s:%d", groupID, len(channels)))
		if shadowGroupID == "" && len(channels) > 0 {
			shadowGroupID = groupID
		}
	}
	comparison := "incomparable"
	switch {
	case task.ActualSource != model.ChannelCandidateSourcePlatform:
		comparison = "incomparable"
	case shadowGroupID == "":
		comparison = "no_shadow_candidate"
	case shadowGroupID == task.ActualGroupID:
		comparison = "match"
	default:
		comparison = "different"
	}
	log("ROUTE_SHADOW outcome=evaluated comparison=%s trace_id=%s token_id=%s actual_group_id=%s actual_channel_id=%s actual_source=%s shadow_group_id=%s ordered_groups=%s group_candidate_counts=%s endpoint_filtered=%d provider_filtered=%d queue_ms=%d elapsed_ms=%d",
		comparison,
		task.TraceID,
		task.TokenID,
		task.ActualGroupID,
		task.ActualChannelID,
		task.ActualSource,
		shadowGroupID,
		strings.Join(orderedGroups, ","),
		strings.Join(groupCounts, ","),
		endpointFiltered,
		providerFiltered,
		time.Since(task.EnqueuedAt).Milliseconds()-time.Since(startedAt).Milliseconds(),
		time.Since(startedAt).Milliseconds(),
	)
}

func orderEntitledGroupsByPreference(preference []string, sources []model.UserEntitlementSource) []string {
	eligible := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		groupID := strings.TrimSpace(source.GroupID)
		if groupID != "" {
			eligible[groupID] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(eligible))
	seen := make(map[string]struct{}, len(eligible))
	appendIfEligible := func(rawGroupID string) {
		groupID := strings.TrimSpace(rawGroupID)
		if groupID == "" {
			return
		}
		if _, ok := eligible[groupID]; !ok {
			return
		}
		if _, ok := seen[groupID]; ok {
			return
		}
		seen[groupID] = struct{}{}
		ordered = append(ordered, groupID)
	}
	for _, groupID := range preference {
		appendIfEligible(groupID)
	}
	for _, source := range sources {
		appendIfEligible(source.GroupID)
	}
	return ordered
}
