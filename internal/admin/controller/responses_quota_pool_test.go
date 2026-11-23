package controller

import (
	"net/http"
	"testing"

	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/relay/model"
)

func TestResponsesQuotaPoolReportsOnlyConfirmedExhaustion(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		pinned     bool
		attempted  bool
		want429    bool
	}{
		{name: "all quota", code: "credit_balance_exhausted", attempted: true, want429: true},
		{name: "other eligible channel untried", code: "credit_balance_exhausted"},
		{name: "mixed transient", code: "slow_down", attempted: true},
		{name: "explicit continuity still pinned", code: "credit_balance_exhausted", attempted: true, pinned: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := quotaReplayContext()
			c.Set(ctxkey.ResponsesStatefulRequest, tc.pinned)
			pool := responsesQuotaPool{"B": {}}
			final := &model.ErrorWithStatusCode{StatusCode: http.StatusTooManyRequests, Error: model.Error{Code: tc.code}}
			if tc.attempted {
				pool = pool.afterFailure("B", final)
			}
			if got := pool.reportExhaustion(c, final); got != tc.want429 {
				t.Fatalf("reportExhaustion = %v, want %v", got, tc.want429)
			}
			if tc.want429 && (final.StatusCode != http.StatusTooManyRequests || final.Type != "usage_limit_reached" || final.Code != "usage_limit_reached") {
				t.Fatalf("unexpected Codex quota response: %+v", final)
			}
		})
	}
}
