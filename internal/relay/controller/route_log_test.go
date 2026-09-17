package controller

import (
	"testing"

	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/meta"
)

func TestApplyRouteObservabilityToLogRecordsSelectedProvider(t *testing.T) {
	entry := &adminmodel.Log{ModelName: "gpt-5.6"}
	routeMeta := &meta.Meta{
		OriginModelName:     "gpt-5.6",
		UpstreamRequestPath: "/v1/responses",
		ChannelModelConfigs: []adminmodel.ChannelModel{{
			Model:         "gpt-5.6",
			UpstreamModel: "gpt-5.6-upstream",
			Provider:      " OpenAI ",
			Selected:      true,
		}},
	}

	applyRouteObservabilityToLog(entry, routeMeta, "gpt-5.6-upstream")

	if entry.Provider != "openai" {
		t.Fatalf("Provider=%q, want normalized selected provider", entry.Provider)
	}
	if entry.RequestModelName != "gpt-5.6" || entry.ActualModelName != "gpt-5.6-upstream" {
		t.Fatalf("unexpected model snapshots: request=%q actual=%q", entry.RequestModelName, entry.ActualModelName)
	}
}

func TestApplyRouteObservabilityToLogLeavesProviderEmptyWhenRouteIsUnknown(t *testing.T) {
	entry := &adminmodel.Log{ModelName: "gpt-5.6"}
	applyRouteObservabilityToLog(entry, &meta.Meta{OriginModelName: "gpt-5.6"}, "gpt-5.6")
	if entry.Provider != "" {
		t.Fatalf("Provider=%q, want empty provider for unknown route", entry.Provider)
	}
}
