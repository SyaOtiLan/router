package model

import (
	"context"
	"strings"
)

// AllResponsesCandidatesQuotaDisabled is only used when normal selection
// found no route. Absence of an endpoint, a mixed reason, or any failed
// lookup leaves the reason unknown.
func AllResponsesCandidatesQuotaDisabled(ctx context.Context, group, modelName string, channels []*ChannelCandidate) bool {
	if DB == nil || len(channels) == 0 {
		return false
	}
	db := DB.WithContext(ctx)
	if db.Migrator().HasTable(&GroupSupplyBinding{}) {
		var count int64
		err := db.Model(&GroupSupplyBinding{}).Where(`"group" = ? AND model = ? AND enabled = ?`, group, modelName, true).Count(&count).Error
		if err != nil || count > 0 {
			return false
		}
	}
	ids := make([]string, 0, len(channels))
	lookup := make(map[string][]string, len(channels))
	var aliases []string
	for _, channel := range channels {
		if channel == nil || strings.TrimSpace(channel.Id) == "" || IsPersonalChannelCandidateID(channel.Id) || IsCommunityOfferChannelID(channel.Id) {
			return false
		}
		ids = append(ids, channel.Id)
		lookup[channel.Id] = endpointLookupCandidatesForChannel(group, modelName, channel.Id, nil)
		aliases = append(aliases, lookup[channel.Id]...)
	}
	var rows []ChannelModelEndpoint
	if err := db.Where("channel_id IN ? AND model IN ?", ids, normalizeTrimmedValuesPreserveOrder(aliases)).Find(&rows).Error; err != nil {
		return false
	}
	return responsesEndpointsHaveQuotaMarkers(lookup, rows)
}

func responsesEndpointsHaveQuotaMarkers(lookup map[string][]string, rows []ChannelModelEndpoint) bool {
	type endpointKey struct{ channel, model string }
	type endpointState struct{ hasRows, quotaDisabled, otherResponses bool }
	states := make(map[endpointKey]endpointState)
	for _, row := range rows {
		key := endpointKey{row.ChannelId, row.Model}
		state := states[key]
		state.hasRows = true
		if NormalizeRequestedChannelModelEndpoint(row.Endpoint) == ChannelModelEndpointResponses {
			if !row.Enabled && row.DisabledBy == ChannelEndpointDisabledByRuntimeQuota {
				state.quotaDisabled = true
			} else {
				state.otherResponses = true
			}
		}
		states[key] = state
	}
	for channel, aliases := range lookup {
		var state endpointState
		for _, alias := range aliases {
			state = states[endpointKey{channel, alias}]
			if state.hasRows {
				break
			}
		}
		if !state.quotaDisabled || state.otherResponses {
			return false
		}
	}
	return len(lookup) > 0
}
