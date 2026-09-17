package billing

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestShouldAutoRefreshChannelBillingSkipsInsufficientBalanceAutoDisabled(t *testing.T) {
	channel := &model.Channel{Id: "channel-1", Status: model.ChannelStatusAutoDisabled}

	if shouldAutoRefreshChannelBilling(channel) {
		t.Fatalf("insufficient-balance auto-disabled channel should not be auto-refreshed")
	}
}

func TestShouldAutoRefreshChannelBillingIncludesEnabled(t *testing.T) {
	channel := &model.Channel{Id: "channel-1", Status: model.ChannelStatusEnabled}

	if !shouldAutoRefreshChannelBilling(channel) {
		t.Fatalf("enabled channel should be auto-refreshed")
	}
}

func TestShouldAutoRefreshChannelBillingSkipsManualDisabled(t *testing.T) {
	channel := &model.Channel{Id: "channel-1", Status: model.ChannelStatusManuallyDisabled}

	if shouldAutoRefreshChannelBilling(channel) {
		t.Fatalf("manually disabled channel should not be auto-refreshed")
	}
}

func TestShouldAutoRefreshChannelBillingSkipsOtherAutoDisabled(t *testing.T) {
	channel := &model.Channel{Id: "channel-1", Status: model.ChannelStatusAutoDisabled}
	if shouldAutoRefreshChannelBilling(channel) {
		t.Fatalf("non-insufficient-balance auto-disabled channel should not be auto-refreshed")
	}
}

func TestSupportsProviderUsageSync(t *testing.T) {
	for _, source := range []string{"", "manual", "unsupported", "builtin_balance"} {
		if supportsProviderUsageSync(source) {
			t.Fatalf("supportsProviderUsageSync(%q) = true", source)
		}
	}
	for _, source := range []string{"aixhan", "AIXHAN", "custom_adapter"} {
		if !supportsProviderUsageSync(source) {
			t.Fatalf("supportsProviderUsageSync(%q) = false", source)
		}
	}
}
