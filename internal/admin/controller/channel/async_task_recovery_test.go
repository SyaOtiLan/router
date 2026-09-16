package channel

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestResolveChannelModelTestTaskTargetsIncludesRuntimeDisabledExplicitModel(t *testing.T) {
	channel := &model.Channel{}
	channel.SetChannelModels([]model.ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.6-sol",
			UpstreamModel: "gpt-5.6-sol",
			Selected:      false,
			Type:          model.ProviderModelTypeText,
			Endpoints:     []string{model.ChannelModelEndpointResponses},
		},
	})

	targets := resolveChannelModelTestTaskTargets(channel, channelModelTestModeSingle, "gpt-5.6-sol", nil, []channelModelTestTargetItem{{
		Model:    "gpt-5.6-sol",
		Endpoint: model.ChannelModelEndpointResponses,
	}})
	if len(targets) != 1 {
		t.Fatalf("targets=%d, want 1", len(targets))
	}
	if targets[0].Row.Model != "gpt-5.6-sol" || targets[0].Row.Endpoint != model.ChannelModelEndpointResponses {
		t.Fatalf("target=%+v", targets[0].Row)
	}
}
