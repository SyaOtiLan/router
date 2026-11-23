package model

import "testing"

func TestResponsesQuotaMarkersRequireEveryCandidate(t *testing.T) {
	lookup := map[string][]string{"A": {"mapped", "alias"}, "B": {"model"}}
	a := ChannelModelEndpoint{ChannelId: "A", Model: "mapped", Endpoint: ChannelModelEndpointResponses, DisabledBy: ChannelEndpointDisabledByRuntimeQuota}
	b := ChannelModelEndpoint{ChannelId: "B", Model: "model", Endpoint: ChannelModelEndpointResponses, DisabledBy: ChannelEndpointDisabledByRuntimeQuota}
	for _, tc := range []struct {
		name string
		rows []ChannelModelEndpoint
		want bool
	}{
		{name: "both marked", rows: []ChannelModelEndpoint{a, b}, want: true},
		{name: "missing candidate", rows: []ChannelModelEndpoint{a}},
		{name: "different disable reason", rows: []ChannelModelEndpoint{a, {ChannelId: "B", Model: "model", Endpoint: ChannelModelEndpointResponses, DisabledBy: "runtime"}}},
		{name: "recovered", rows: []ChannelModelEndpoint{a, {ChannelId: "B", Model: "model", Endpoint: ChannelModelEndpointResponses, Enabled: true}}},
		{name: "unselected alias", rows: []ChannelModelEndpoint{b,
			{ChannelId: "A", Model: "mapped", Endpoint: ChannelModelEndpointChat, Enabled: true},
			{ChannelId: "A", Model: "alias", Endpoint: ChannelModelEndpointResponses, DisabledBy: ChannelEndpointDisabledByRuntimeQuota}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := responsesEndpointsHaveQuotaMarkers(lookup, tc.rows); got != tc.want {
				t.Fatalf("responsesEndpointsHaveQuotaMarkers = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResponsesQuotaFailurePreservesOtherDisableReason(t *testing.T) {
	rows := []ChannelModelEndpoint{{ChannelId: "A", Model: "model", Endpoint: ChannelModelEndpointResponses, DisabledBy: "runtime", DisabledReason: "unsupported"}}
	updated, changed := buildDisabledChannelModelEndpointRows(rows, "A", "model", ChannelModelEndpointResponses, "billing exhausted", ChannelEndpointDisabledByRuntimeQuota)
	if changed || len(updated) != 1 || updated[0].DisabledBy != "runtime" || updated[0].DisabledReason != "unsupported" {
		t.Fatalf("quota failure overwrote other disable reason: %+v", updated)
	}
}
