package channel

import "testing"

func TestNormalizeProtocolNameDoesNotRestoreRemovedProtocolAliases(t *testing.T) {
	if got := NormalizeProtocolName("openai-compatible"); got != "openai-compatible" {
		t.Fatalf("NormalizeProtocolName(openai-compatible) = %q", got)
	}
	if got := NormalizeProtocolName("49"); got != "openai" {
		t.Fatalf("NormalizeProtocolName(49) = %q, want default openai", got)
	}
}
