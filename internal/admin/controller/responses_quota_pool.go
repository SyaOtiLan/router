package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	dbmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
	"github.com/yeying-community/router/internal/relay/routing"
)

// A non-nil pool tracks channels that have yet to return a definite quota
// error. An empty candidate selection alone does not prove quota exhaustion.
type responsesQuotaPool map[string]struct{}

func snapshotResponsesQuotaPool(c *gin.Context, first *model.ErrorWithStatusCode, policy routing.ProviderRoutingPolicy, user, group, requestModel, provider, channelID string) responsesQuotaPool {
	if getEffectiveRelayMode(c) != relaymode.Responses || !confirmedUpstreamQuota(first) ||
		policy.RetryScope == routing.RetryScopeNone || c.Writer.Written() || channelID == "" ||
		dbmodel.IsPersonalChannelCandidateID(channelID) || dbmodel.IsCommunityOfferChannelID(channelID) {
		return nil
	}
	if _, explicit := c.Get(ctxkey.SpecificChannelId); explicit {
		return nil
	}
	if dbmodel.NormalizePersonalRoutePolicy(c.GetString(ctxkey.PersonalRoutePolicy)) != dbmodel.PersonalRoutePolicyCommunityOnly {
		personal, err := dbmodel.ListPersonalChannelCandidatesForModel(user, requestModel)
		if err != nil || len(personal) != 0 {
			return nil
		}
	}
	candidates, err := dbmodel.CacheListSatisfiedChannelsForRequest(group, requestModel, c.Request.URL.Path)
	if err != nil {
		return nil
	}
	pool := make(responsesQuotaPool, len(candidates))
	for _, candidate := range candidates {
		if candidate.Id != channelID && retryProviderAllowed(policy, provider, provider, channelProvider(candidate, requestModel)) {
			pool[candidate.Id] = struct{}{}
		}
	}
	return pool
}

func confirmedUpstreamQuota(err *model.ErrorWithStatusCode) bool {
	return err != nil && (err.StatusCode == http.StatusTooManyRequests || err.StatusCode == http.StatusPaymentRequired) &&
		err.Error.IsQuotaExhausted() && !isLocalQuotaRelayError(err)
}

func (pool responsesQuotaPool) afterFailure(channelID string, err *model.ErrorWithStatusCode) responsesQuotaPool {
	if !confirmedUpstreamQuota(err) {
		return nil
	}
	delete(pool, channelID)
	return pool
}

func (pool responsesQuotaPool) reportExhaustion(c *gin.Context, err *model.ErrorWithStatusCode) bool {
	if pool == nil || len(pool) != 0 || !confirmedUpstreamQuota(err) || isPinnedResponsesRequest(c) ||
		c.Writer.Written() || c.Request.Context().Err() != nil {
		return false
	}
	*err = model.ResponsesPoolQuotaError()
	return true
}
