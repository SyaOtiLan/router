package model

import "testing"

func TestProviderCatalogRefresh20261121AddsOfficialModelsAndUpdatesLifecycle(t *testing.T) {
	details := providerMigrationDetailsByProvider(t)
	tests := []struct {
		provider, model, status string
		deleted                 bool
		input, output           float64
		priceUnit               string
	}{
		{"openai", "gpt-6.2-astra", ProviderModelStatusActive, false, 0.008, 0.04, ProviderPriceUnitPer1KTokens},
		{"openai", "gpt-6.2-sol", ProviderModelStatusActive, false, 0.002, 0.01, ProviderPriceUnitPer1KTokens},
		{"openai", "gpt-6.1-terra", ProviderModelStatusActive, false, 0.001, 0.005, ProviderPriceUnitPer1KTokens},
		{"openai", "gpt-image-2.6-sunburst", ProviderModelStatusActive, false, 0.008, 0.03, ProviderPriceUnitPer1KTokens},
		{"openai", "gpt-live-1.1", ProviderModelStatusActive, false, 0.05, 0.02, ProviderPriceUnitPerMinute},
		{"anthropic", "claude-fable-5-2", ProviderModelStatusActive, false, 0.008, 0.04, ProviderPriceUnitPer1KTokens},
		{"qwen", "qwen4-max", ProviderModelStatusActive, false, 0.01, 0.03, ProviderPriceUnitPer1KTokens},
		{"qwen", "qwen3.5-omni-plus", ProviderModelStatusActive, false, 0.007, 0.04, ProviderPriceUnitPer1KTokens},
		{"qwen", "qwen3.7-text-embedding-flash", ProviderModelStatusActive, false, 0.000125, 0, ProviderPriceUnitPer1KTokens},
		{"qwen", "qwen-audio-3.1-realtime-plus", ProviderModelStatusActive, false, 0.005, 0.04, ProviderPriceUnitPer1KTokens},
		{"deepseek", "deepseek-v4-pro", ProviderModelStatusActive, false, 0.00132, 0.00396, ProviderPriceUnitPer1KTokens},
		{"xai", "grok-4.20-0309-reasoning", ProviderModelStatusActive, false, 0.00125, 0.0025, ProviderPriceUnitPer1KTokens},
		{"minimax", "MiniMax-H3-Image", ProviderModelStatusActive, false, 0, 0, ProviderPriceUnitPerImage},
		{"minimax", "MiniMax-M2.7", ProviderModelStatusDeprecated, true, 0, 0, ProviderPriceUnitPer1KTokens},
		{"deepgram", "flux-general-en", ProviderModelStatusActive, false, 0.0065, 0, ProviderPriceUnitPerMinute},
	}
	for _, tt := range tests {
		got, ok := details[tt.provider][tt.model]
		if !ok {
			t.Fatalf("%s/%s missing from merged catalog", tt.provider, tt.model)
		}
		if got.Status != tt.status || got.IsDeleted != tt.deleted || got.InputPrice != tt.input || got.OutputPrice != tt.output || got.PriceUnit != tt.priceUnit {
			t.Fatalf("%s/%s lifecycle/pricing=%q/%t/%v/%v/%q", tt.provider, tt.model, got.Status, got.IsDeleted, got.InputPrice, got.OutputPrice, got.PriceUnit)
		}
	}

	retiredImage := details["openai"]["gpt-image-2"]
	if retiredImage.Status != ProviderModelStatusDeprecated || retiredImage.IsDeleted {
		t.Fatalf("gpt-image-2 lifecycle=%q/%t, want deprecated but available through its announced shutdown date", retiredImage.Status, retiredImage.IsDeleted)
	}
	for _, name := range []string{"MiniMax-M2.5", "MiniMax-M2.5-highspeed", "MiniMax-M2.7", "MiniMax-M2.7-highspeed"} {
		retired := details["minimax"][name]
		if retired.Status != ProviderModelStatusDeprecated || !retired.IsDeleted {
			t.Fatalf("%s lifecycle=%q/%t, want retired and hidden", name, retired.Status, retired.IsDeleted)
		}
	}
	if !ProviderModelTagsContain(details["minimax"]["MiniMax-H3-Image"].Tags, ProviderModelTagNativeAdapterRequired) || len(details["minimax"]["MiniMax-H3-Image"].SupportedEndpoints) != 0 {
		t.Fatal("MiniMax image models must remain catalogued without exposing unsupported Router endpoints")
	}
	if !ProviderModelTagsContain(details["deepgram"]["flux-general-en"].Tags, ProviderModelTagNativeAdapterRequired) || len(details["deepgram"]["flux-general-en"].SupportedEndpoints) != 0 {
		t.Fatal("Deepgram Flux models must remain catalogued without exposing unsupported Router endpoints")
	}
	if got := details["qwen"]["qwen-audio-3.1-tts-plus"]; len(got.SupportedEndpoints) != 0 || !ProviderModelTagsContain(got.Tags, ProviderModelTagNativeAdapterRequired) {
		t.Fatalf("Qwen proprietary Audio TTS model must remain unroutable until an adapter exists: %#v", got)
	}
	if got := details["qwen"]["qwen3-vl-embedding"].SupportedEndpoints; len(got) != 1 || got[0] != ChannelModelEndpointEmbeddings {
		t.Fatalf("Qwen VL embedding endpoints=%#v, want embeddings", got)
	}
	if got := details["deepseek"]["deepseek-v4-pro"].PriceComponents; len(got) != 1 || got[0].Component != ProviderModelPriceComponentTextCacheRead || got[0].InputPrice != 0.000044 {
		t.Fatalf("DeepSeek V4 Pro cache component=%#v", got)
	}
	if got := details["xai"]["grok-4.20-0309-reasoning"].PriceComponents; len(got) != 4 {
		t.Fatalf("Grok 4.20 price component count=%d, want 4", len(got))
	}
}

func TestProviderCatalogRefresh20261121UpsertIsIdempotentAndDoesNotEnableChannels(t *testing.T) {
	db := newProviderMigrationTestDB(t)
	providers := []string{"openai", "anthropic", "qwen", "deepseek", "minimax", "xai", "deepgram"}
	if err := upsertProviderMigrationProvidersWithDB(db, providers...); err != nil {
		t.Fatalf("first provider catalog refresh: %v", err)
	}
	var modelsBefore, componentsBefore int64
	if err := db.Model(&ProviderModel{}).Where("provider IN ?", providers).Count(&modelsBefore).Error; err != nil {
		t.Fatalf("count models: %v", err)
	}
	if err := db.Model(&ProviderModelPriceComponent{}).Where("provider IN ?", providers).Count(&componentsBefore).Error; err != nil {
		t.Fatalf("count price components: %v", err)
	}
	if err := upsertProviderMigrationProvidersWithDB(db, providers...); err != nil {
		t.Fatalf("second provider catalog refresh: %v", err)
	}
	var modelsAfter, componentsAfter int64
	if err := db.Model(&ProviderModel{}).Where("provider IN ?", providers).Count(&modelsAfter).Error; err != nil {
		t.Fatalf("count refreshed models: %v", err)
	}
	if err := db.Model(&ProviderModelPriceComponent{}).Where("provider IN ?", providers).Count(&componentsAfter).Error; err != nil {
		t.Fatalf("count refreshed price components: %v", err)
	}
	if modelsAfter != modelsBefore || componentsAfter != componentsBefore {
		t.Fatalf("catalog refresh changed counts on second run: models %d -> %d, components %d -> %d", modelsBefore, modelsAfter, componentsBefore, componentsAfter)
	}
	if db.Migrator().HasTable(&ChannelModel{}) {
		t.Fatal("provider catalog refresh must not create or enable channel models")
	}
}
