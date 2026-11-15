package channel

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestShouldProbeInsufficientBalanceRecovery(t *testing.T) {
	channel := &model.PlatformChannel{Id: "channel-1", Status: model.ChannelStatusAutoDisabled}
	state := model.ChannelCircuitBreakerState{
		ChannelId: "channel-1",
		State:     model.ChannelCircuitBreakerStateCanceled,
		Reason:    model.ChannelCircuitBreakerReasonInsufficientBalance,
	}

	if !shouldProbeInsufficientBalanceRecovery(channel, state) {
		t.Fatalf("insufficient-balance auto-disabled channel should schedule recovery probe")
	}
}

func TestShouldProbeInsufficientBalanceRecoverySkipsManualDisabled(t *testing.T) {
	channel := &model.PlatformChannel{Id: "channel-1", Status: model.ChannelStatusManuallyDisabled}
	state := model.ChannelCircuitBreakerState{
		ChannelId: "channel-1",
		State:     model.ChannelCircuitBreakerStateCanceled,
		Reason:    model.ChannelCircuitBreakerReasonInsufficientBalance,
	}

	if shouldProbeInsufficientBalanceRecovery(channel, state) {
		t.Fatalf("manual disabled channel should not schedule recovery probe")
	}
}

func TestShouldProbeInsufficientBalanceRecoverySkipsOtherAutoDisabledReason(t *testing.T) {
	channel := &model.PlatformChannel{Id: "channel-1", Status: model.ChannelStatusAutoDisabled}
	state := model.ChannelCircuitBreakerState{
		ChannelId: "channel-1",
		State:     model.ChannelCircuitBreakerStateCanceled,
		Reason:    "permission denied",
	}

	if shouldProbeInsufficientBalanceRecovery(channel, state) {
		t.Fatalf("non-insufficient-balance auto-disabled channel should not schedule recovery probe")
	}
}

func TestShouldProbeInsufficientBalanceRecoveryDoesNotRequireBillingEntitlement(t *testing.T) {
	channel := &model.PlatformChannel{Id: "channel-1", Status: model.ChannelStatusAutoDisabled}
	state := model.ChannelCircuitBreakerState{
		ChannelId: "channel-1",
		State:     model.ChannelCircuitBreakerStateCanceled,
		Reason:    model.ChannelCircuitBreakerReasonInsufficientBalance,
	}

	if !shouldProbeInsufficientBalanceRecovery(channel, state) {
		t.Fatalf("insufficient-balance recovery probe should not depend on billing entitlement")
	}
}
