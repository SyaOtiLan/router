package model

import (
	"math/rand"
	"strings"
)

type SatisfiedChannelSelectionStats struct {
	TotalCandidates        int
	RemainingCandidates    int
	SelectedPriority       int64
	SelectedTierCandidates int
	SelectionScope         string
}

func SelectRandomSatisfiedChannel(channels []*ChannelCandidate, ignoreFirstPriority bool, excludedChannelIDs map[string]struct{}) *ChannelCandidate {
	channel, _ := SelectRandomSatisfiedChannelWithStats(channels, ignoreFirstPriority, excludedChannelIDs)
	return channel
}

func SelectRandomSatisfiedChannelWithStats(channels []*ChannelCandidate, ignoreFirstPriority bool, excludedChannelIDs map[string]struct{}) (*ChannelCandidate, SatisfiedChannelSelectionStats) {
	stats := SatisfiedChannelSelectionStats{
		TotalCandidates: len(channels),
	}
	if len(channels) == 0 {
		return nil, stats
	}
	targets := channels
	if ignoreFirstPriority {
		startIdx := nextPriorityStartIndex(channels)
		if startIdx >= len(channels) {
			stats.SelectionScope = "lower_priority_only"
			return nil, stats
		}
		targets = channels[startIdx:]
	}
	filtered := filterExcludedChannels(targets, excludedChannelIDs)
	stats.RemainingCandidates = len(filtered)
	if len(filtered) == 0 {
		if ignoreFirstPriority {
			stats.SelectionScope = "lower_priority_only"
		} else {
			stats.SelectionScope = "candidate_exhausted"
		}
		return nil, stats
	}
	endIdx := len(filtered)
	firstPriority := filtered[0].GetPriority()
	stats.SelectedPriority = firstPriority
	for i := range filtered {
		if filtered[i].GetPriority() != firstPriority {
			endIdx = i
			break
		}
	}
	stats.SelectedTierCandidates = endIdx
	switch {
	case ignoreFirstPriority:
		stats.SelectionScope = "lower_priority_only"
	case firstPriority != channels[0].GetPriority():
		stats.SelectionScope = "downgraded"
	default:
		stats.SelectionScope = "same_priority"
	}
	return selectWeightedChannel(filtered[:endIdx]), stats
}

func selectWeightedChannel(channels []*ChannelCandidate) *ChannelCandidate {
	if len(channels) == 0 {
		return nil
	}
	if len(channels) == 1 {
		return channels[0]
	}
	var totalWeight uint64
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		totalWeight += uint64(channel.GetWeight())
	}
	if totalWeight == 0 {
		return channels[rand.Intn(len(channels))]
	}
	target := uint64(rand.Int63n(int64(totalWeight)))
	var cumulative uint64
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		cumulative += uint64(channel.GetWeight())
		if target < cumulative {
			return channel
		}
	}
	return channels[len(channels)-1]
}

func filterExcludedChannels(channels []*ChannelCandidate, excludedChannelIDs map[string]struct{}) []*ChannelCandidate {
	if len(channels) == 0 {
		return nil
	}
	if len(excludedChannelIDs) == 0 {
		result := make([]*ChannelCandidate, 0, len(channels))
		result = append(result, channels...)
		return result
	}
	result := make([]*ChannelCandidate, 0, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		if _, excluded := excludedChannelIDs[channelID]; excluded {
			continue
		}
		result = append(result, channel)
	}
	return result
}

func nextPriorityStartIndex(channels []*ChannelCandidate) int {
	if len(channels) == 0 {
		return 0
	}
	endIdx := len(channels)
	firstPriority := channels[0].GetPriority()
	if firstPriority > 0 {
		for i := range channels {
			if channels[i].GetPriority() != firstPriority {
				endIdx = i
				break
			}
		}
	}
	return endIdx
}
