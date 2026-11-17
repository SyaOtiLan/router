package middleware

import (
	"reflect"
	"testing"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
)

func TestOrderEntitledGroupsByPreferenceIntersectsCurrentEntitlements(t *testing.T) {
	sources := []model.UserEntitlementSource{
		{GroupID: "group-b"},
		{GroupID: "group-a"},
		{GroupID: "group-b"},
		{GroupID: "group-c"},
	}

	got := orderEntitledGroupsByPreference([]string{"group-c", "stale-group", "group-a", "group-c"}, sources)
	want := []string{"group-c", "group-a", "group-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orderEntitledGroupsByPreference() = %#v, want %#v", got, want)
	}
}

func TestGroupRouteShadowRequiresEnabledAllowlistedTokenAndSample(t *testing.T) {
	previousEnabled := config.GroupRouteShadowEnabled
	previousAllowlist := config.GroupRouteShadowTokenAllowlist
	previousRate := config.GroupRouteShadowSampleRate
	t.Cleanup(func() {
		config.GroupRouteShadowEnabled = previousEnabled
		config.GroupRouteShadowTokenAllowlist = previousAllowlist
		config.GroupRouteShadowSampleRate = previousRate
	})

	config.GroupRouteShadowEnabled = true
	config.GroupRouteShadowTokenAllowlist = []string{"internal-token"}
	config.GroupRouteShadowSampleRate = 1
	if !groupRouteShadowEnabledForToken("internal-token") {
		t.Fatal("allowlisted token at 100% sampling should be enabled")
	}
	if groupRouteShadowEnabledForToken("other-token") {
		t.Fatal("token outside allowlist should be disabled")
	}
	if groupRouteShadowEnabledForToken("") {
		t.Fatal("empty token ID should be disabled")
	}

	config.GroupRouteShadowTokenAllowlist = nil
	if groupRouteShadowEnabledForToken("internal-token") {
		t.Fatal("empty allowlist should fail closed")
	}
	config.GroupRouteShadowTokenAllowlist = []string{"internal-token"}
	config.GroupRouteShadowSampleRate = 0
	if groupRouteShadowEnabledForToken("internal-token") {
		t.Fatal("zero sampling rate should be disabled")
	}
}
