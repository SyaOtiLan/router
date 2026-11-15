package model

import (
	"math/rand"
	"testing"
)

func TestSelectRandomSatisfiedChannelExcludesFailedChannels(t *testing.T) {
	channels := platformChannelCandidates([]*PlatformChannel{
		{Id: "channel-a"},
		{Id: "channel-b"},
		{Id: "channel-c"},
	})

	got := SelectRandomSatisfiedChannel(channels, false, map[string]struct{}{
		"channel-a": {},
		"channel-b": {},
	})

	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-c" {
		t.Fatalf("unexpected channel id: got %q want %q", got.Id, "channel-c")
	}
}

func TestSelectRandomSatisfiedChannelIgnoresFirstPriorityLayerWhenRequested(t *testing.T) {
	high := int64(10)
	low := int64(1)
	channels := platformChannelCandidates([]*PlatformChannel{
		{Id: "channel-a", Priority: &high},
		{Id: "channel-b", Priority: &high},
		{Id: "channel-c", Priority: &low},
	})

	got := SelectRandomSatisfiedChannel(channels, true, nil)

	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-c" {
		t.Fatalf("unexpected channel id: got %q want %q", got.Id, "channel-c")
	}
}

func TestSelectRandomSatisfiedChannelKeepsCurrentPriorityTierBeforeDowngrading(t *testing.T) {
	high := int64(10)
	low := int64(1)
	channels := platformChannelCandidates([]*PlatformChannel{
		{Id: "channel-a", Priority: &high},
		{Id: "channel-b", Priority: &high},
		{Id: "channel-c", Priority: &low},
	})

	got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, map[string]struct{}{
		"channel-a": {},
	})

	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-b" {
		t.Fatalf("unexpected channel id: got %q want %q", got.Id, "channel-b")
	}
	if stats.SelectionScope != "same_priority" {
		t.Fatalf("unexpected selection scope: got %q", stats.SelectionScope)
	}
	if stats.SelectedTierCandidates != 1 {
		t.Fatalf("unexpected selected tier candidates: got %d want %d", stats.SelectedTierCandidates, 1)
	}
	if stats.RemainingCandidates != 2 {
		t.Fatalf("unexpected remaining candidates: got %d want %d", stats.RemainingCandidates, 2)
	}
}

func TestSelectRandomSatisfiedChannelDowngradesWhenCurrentPriorityTierExhausted(t *testing.T) {
	high := int64(10)
	low := int64(1)
	channels := platformChannelCandidates([]*PlatformChannel{
		{Id: "channel-a", Priority: &high},
		{Id: "channel-b", Priority: &high},
		{Id: "channel-c", Priority: &low},
	})

	got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, map[string]struct{}{
		"channel-a": {},
		"channel-b": {},
	})

	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-c" {
		t.Fatalf("unexpected channel id: got %q want %q", got.Id, "channel-c")
	}
	if stats.SelectionScope != "downgraded" {
		t.Fatalf("unexpected selection scope: got %q", stats.SelectionScope)
	}
	if stats.SelectedTierCandidates != 1 {
		t.Fatalf("unexpected selected tier candidates: got %d want %d", stats.SelectedTierCandidates, 1)
	}
	if stats.RemainingCandidates != 1 {
		t.Fatalf("unexpected remaining candidates: got %d want %d", stats.RemainingCandidates, 1)
	}
}

func TestSelectRandomSatisfiedChannelReturnsNilWhenAllTiersExhausted(t *testing.T) {
	channels := platformChannelCandidates([]*PlatformChannel{
		{Id: "channel-a"},
		{Id: "channel-b"},
	})

	got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, map[string]struct{}{
		"channel-a": {},
		"channel-b": {},
	})

	if got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
	if stats.SelectionScope != "candidate_exhausted" {
		t.Fatalf("unexpected selection scope: got %q", stats.SelectionScope)
	}
	if stats.RemainingCandidates != 0 {
		t.Fatalf("unexpected remaining candidates: got %d want %d", stats.RemainingCandidates, 0)
	}
}

func TestSelectRandomSatisfiedChannelUsesRoutePriorityOverride(t *testing.T) {
	basePriority := int64(0)
	channels := []*ChannelCandidate{
		CloneChannelWithPriority(&ChannelCandidate{PlatformChannel: PlatformChannel{Id: "channel-a", Priority: &basePriority}}, 1),
		CloneChannelWithPriority(&ChannelCandidate{PlatformChannel: PlatformChannel{Id: "channel-b", Priority: &basePriority}}, 1),
		CloneChannelWithPriority(&ChannelCandidate{PlatformChannel: PlatformChannel{Id: "channel-c", Priority: &basePriority}}, 0),
	}

	got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, nil)
	if got == nil {
		t.Fatalf("expected channel, got nil")
	}
	if got.Id != "channel-a" && got.Id != "channel-b" {
		t.Fatalf("unexpected channel id: got %q want tier-1 candidate", got.Id)
	}
	if stats.SelectedPriority != 1 {
		t.Fatalf("selected priority = %d, want 1", stats.SelectedPriority)
	}
	if stats.SelectedTierCandidates != 2 {
		t.Fatalf("selected tier candidates = %d, want 2", stats.SelectedTierCandidates)
	}
}

func TestResolveRuntimeChannelPriorityDemotesHalfOpen(t *testing.T) {
	priority := resolveRuntimeChannelPriority(NewPlatformChannelCandidate(&PlatformChannel{Status: ChannelStatusHalfOpen}), 100)
	if priority != ChannelHalfOpenPriority {
		t.Fatalf("half-open priority = %d, want %d", priority, ChannelHalfOpenPriority)
	}
	enabledPriority := resolveRuntimeChannelPriority(NewPlatformChannelCandidate(&PlatformChannel{Status: ChannelStatusEnabled}), 100)
	if enabledPriority != 100 {
		t.Fatalf("enabled priority = %d, want 100", enabledPriority)
	}
}

func TestChannelGetWeightDefaultsZeroAndNilToOne(t *testing.T) {
	var zero uint
	if got := (&PlatformChannel{}).GetWeight(); got != 1 {
		t.Fatalf("nil weight = %d, want 1", got)
	}
	if got := (&PlatformChannel{Weight: &zero}).GetWeight(); got != 1 {
		t.Fatalf("zero weight = %d, want 1", got)
	}
	custom := uint(7)
	if got := (&PlatformChannel{Weight: &custom}).GetWeight(); got != 7 {
		t.Fatalf("custom weight = %d, want 7", got)
	}
}

func TestSelectRandomSatisfiedChannelUsesWeightWithinSamePriorityTier(t *testing.T) {
	rand.Seed(1)
	priority := int64(10)
	heavyWeight := uint(9)
	lightWeight := uint(1)
	channels := platformChannelCandidates([]*PlatformChannel{
		{Id: "channel-heavy", Priority: &priority, Weight: &heavyWeight},
		{Id: "channel-light", Priority: &priority, Weight: &lightWeight},
	})
	counts := map[string]int{
		"channel-heavy": 0,
		"channel-light": 0,
	}

	for i := 0; i < 2000; i++ {
		got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, nil)
		if got == nil {
			t.Fatalf("expected channel, got nil")
		}
		counts[got.Id]++
		if stats.SelectedPriority != priority {
			t.Fatalf("selected priority = %d, want %d", stats.SelectedPriority, priority)
		}
	}

	if counts["channel-heavy"] <= counts["channel-light"] {
		t.Fatalf("weighted selection ignored: heavy=%d light=%d", counts["channel-heavy"], counts["channel-light"])
	}
}

func TestSelectRandomSatisfiedChannelIgnoresExhaustedHighPriorityAndUsesWeightOnFallbackTier(t *testing.T) {
	rand.Seed(2)
	high := int64(10)
	low := int64(1)
	heavyWeight := uint(9)
	lightWeight := uint(1)
	channels := platformChannelCandidates([]*PlatformChannel{
		{Id: "channel-a", Priority: &high},
		{Id: "channel-b", Priority: &high},
		{Id: "channel-c", Priority: &low, Weight: &heavyWeight},
		{Id: "channel-d", Priority: &low, Weight: &lightWeight},
	})
	counts := map[string]int{
		"channel-c": 0,
		"channel-d": 0,
	}

	for i := 0; i < 1000; i++ {
		got, stats := SelectRandomSatisfiedChannelWithStats(channels, false, map[string]struct{}{
			"channel-a": {},
			"channel-b": {},
		})
		if got == nil {
			t.Fatalf("expected channel, got nil")
		}
		if got.Id != "channel-c" && got.Id != "channel-d" {
			t.Fatalf("unexpected fallback channel id: %q", got.Id)
		}
		if stats.SelectionScope != "downgraded" {
			t.Fatalf("unexpected selection scope: got %q", stats.SelectionScope)
		}
		counts[got.Id]++
	}

	if counts["channel-c"] <= counts["channel-d"] {
		t.Fatalf("fallback weighted selection ignored: c=%d d=%d", counts["channel-c"], counts["channel-d"])
	}
}

func platformChannelCandidates(channels []*PlatformChannel) []*ChannelCandidate {
	candidates := make([]*ChannelCandidate, 0, len(channels))
	for _, channel := range channels {
		candidates = append(candidates, NewPlatformChannelCandidate(channel))
	}
	return candidates
}
