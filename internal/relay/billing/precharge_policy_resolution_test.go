package billing

import (
	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"testing"
)

func TestResolvePrechargePolicyUsesModelSpecAndGlobalDefault(t *testing.T) {
	resolved := ResolvePrechargePolicy(&adminmodel.ProviderModelSpecification{Version: 1, Billing: &adminmodel.ProviderModelBillingSpecification{Version: 2, PrechargePolicy: PrechargeFixedReserve, MinimumReserve: 900}}, 500)
	if resolved.Source != "model_specification" || resolved.Version != "billing-v2" || resolved.Policy.MinimumReserve != 900 {
		t.Fatalf("resolved=%+v", resolved)
	}
	defaultResolution := ResolvePrechargePolicy(nil, 500)
	if defaultResolution.Source != "global_default" || defaultResolution.Policy.MinimumReserve != 500 || defaultResolution.Policy.Type != PrechargeTokenizerEstimate {
		t.Fatalf("default resolution=%+v", defaultResolution)
	}
}
