package channel

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
)

func TestReconcileChannelProviderUsagePairsOneToOne(t *testing.T) {
	records := []model.ChannelProviderUsageRecord{
		{UpstreamRecordId: "provider-1", OccurredAt: 100, Model: "gpt-5.6-sol", InputTokens: 10, OutputTokens: 2, Currency: "USD"},
		{UpstreamRecordId: "provider-2", OccurredAt: 101, Model: "gpt-5.6-sol", InputTokens: 20, OutputTokens: 3, Currency: "USD"},
	}
	logs := []model.Log{
		{Id: "router-1", CreatedAt: 100, ActualModelName: "gpt-5.6-sol", PromptTokens: 10, CompletionTokens: 2},
		{Id: "router-2", CreatedAt: 101, ActualModelName: "gpt-5.6-sol", PromptTokens: 20, CompletionTokens: 3},
	}
	summary, items := reconcileChannelProviderUsage(records, logs, 1000)
	if summary.PairedCount != 2 || summary.MatchedCount != 2 || len(items) != 2 {
		t.Fatalf("summary=%+v items=%+v", summary, items)
	}
	if items[0].RouterRequestLogID == items[1].RouterRequestLogID {
		t.Fatalf("provider records were matched to the same request: %+v", items)
	}
}

func TestReconcileChannelProviderUsageReportsTokenMismatch(t *testing.T) {
	records := []model.ChannelProviderUsageRecord{{UpstreamRecordId: "provider-1", OccurredAt: 100, Model: "gpt-5", InputTokens: 11, OutputTokens: 2}}
	logs := []model.Log{{Id: "router-1", CreatedAt: 100, RequestModelName: "gpt-5", PromptTokens: 10, CompletionTokens: 2}}
	summary, items := reconcileChannelProviderUsage(records, logs, 1000)
	if summary.UsageMismatchCount != 1 || len(items) != 1 || items[0].Status != channelUsageReconciliationUsageMismatch {
		t.Fatalf("summary=%+v items=%+v", summary, items)
	}
	if items[0].InputTokenDelta != 1 {
		t.Fatalf("input delta=%d, want 1", items[0].InputTokenDelta)
	}
}

func TestReconcileChannelProviderUsageMarksRecentUnmatchedAsPending(t *testing.T) {
	records := []model.ChannelProviderUsageRecord{{UpstreamRecordId: "provider-1", OccurredAt: 990, Model: "gpt-5"}}
	logs := []model.Log{{Id: "router-1", CreatedAt: 995, ActualModelName: "different-model"}}
	summary, items := reconcileChannelProviderUsage(records, logs, 1000)
	if summary.PendingCount != 2 || len(items) != 2 {
		t.Fatalf("summary=%+v items=%+v", summary, items)
	}
	for _, item := range items {
		if item.Status != channelUsageReconciliationPending {
			t.Fatalf("item status=%q, want pending: %+v", item.Status, item)
		}
	}
}

func TestReconcileChannelProviderUsageDoesNotCompareDifferentCurrencies(t *testing.T) {
	records := []model.ChannelProviderUsageRecord{{UpstreamRecordId: "provider-1", OccurredAt: 100, Model: "gpt-5", InputTokens: 1, OutputTokens: 1, CostAmount: 99, Currency: "USD"}}
	logs := []model.Log{{Id: "router-1", CreatedAt: 100, ActualModelName: "gpt-5", PromptTokens: 1, CompletionTokens: 1, BillingProcurementCostBaseAmount: 1, BillingProcurementCostStatus: model.ProcurementCostAttributionStatusActual}}
	summary, items := reconcileChannelProviderUsage(records, logs, 1000)
	if summary.AmountMismatchCount != 0 || len(items) != 1 || items[0].AmountComparable {
		t.Fatalf("summary=%+v items=%+v", summary, items)
	}
}
