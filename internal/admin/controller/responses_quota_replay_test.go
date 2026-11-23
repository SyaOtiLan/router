package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	relaycontroller "github.com/yeying-community/router/internal/relay/controller"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/responsestate"
	"github.com/yeying-community/router/internal/relay/routing"
)

const quotaReplayBody = `{"model":"test","input":[{"role":"user","content":"Read"},{"type":"function_call","id":"fc_A","call_id":"call_1","name":"read","arguments":"{}"},{"type":"function_call_output","id":"fco_A","call_id":"call_1","output":"VALUE"}],"tools":[{"type":"function","name":"read"}]}`

func quotaReplayContext() (*gin.Context, *relaymodel.ErrorWithStatusCode) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(quotaReplayBody))
	c.Set(ctxkey.ResponsesStatefulRequest, true)
	c.Set(ctxkey.ResponsesItemIDs, []string{"fc_A", "fco_A"})
	c.Set(ctxkey.ChannelId, "platform-A")
	err := &relaymodel.ErrorWithStatusCode{StatusCode: http.StatusTooManyRequests, Error: relaymodel.Error{Type: "insufficient_quota", Code: "insufficient_quota"}}
	return c, err
}

func TestResponsesQuotaReplayUnlocksOnlyCompleteToolHistory(t *testing.T) {
	c, err := quotaReplayContext()
	if shouldRetry(c, err) {
		t.Fatal("baseline pin unexpectedly retryable")
	}
	if !prepareResponsesQuotaReplay(c, err) || !shouldRetry(c, err) {
		t.Fatal("complete tool history not made eligible for normal retry")
	}
	raw, _ := io.ReadAll(c.Request.Body)
	state := responsestate.AnalyzeRequestBody(raw)
	if len(state.ItemIDs) != 0 || state.PreviousResponseID != "" || !state.HasToolOutput ||
		!bytes.Equal(raw, c.MustGet(ctxkey.KeyRequestBody).([]byte)) || c.Request.ContentLength != int64(len(raw)) {
		t.Fatal("replay body/context inconsistent")
	}
}

func TestResponsesQuotaReplayRecognizesUpstream429Formats(t *testing.T) {
	for _, tc := range []struct {
		name       string
		errorType  string
		errorCode  any
		wantReplay bool
	}{
		{name: "prepaid balance", errorCode: "credit_balance_exhausted", wantReplay: true},
		{name: "organization spend", errorCode: "organization_spend_limit_exceeded", wantReplay: true},
		{name: "project spend", errorCode: "project_spend_limit_exceeded", wantReplay: true},
		{name: "organization usage", errorCode: "organization_usage_limit_exceeded", wantReplay: true},
		{name: "Codex subscription type", errorType: "usage_limit_reached", wantReplay: true},
		{name: "Codex subscription code", errorCode: "usage_limit_reached", wantReplay: true},
		{name: "temporary throttling", errorType: "rate_limit_error", errorCode: "slow_down"},
		{name: "request rate", errorCode: "rate_limit_exceeded"},
		{name: "usage not included in plan", errorType: "usage_not_included"},
		{name: "Router-local quota", errorType: "insufficient_quota", errorCode: "user_quota_limit_exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := quotaReplayContext()
			body, err := json.Marshal(map[string]relaymodel.Error{"error": {Type: tc.errorType, Code: tc.errorCode}})
			if err != nil {
				t.Fatal(err)
			}
			upstream := relaycontroller.RelayErrorHandler(nil, &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(bytes.NewReader(body)),
			})
			if got := prepareResponsesQuotaReplay(c, upstream); got != tc.wantReplay {
				t.Fatalf("prepareResponsesQuotaReplay = %v, want %v", got, tc.wantReplay)
			}
			if got := shouldRetry(c, upstream); got != tc.wantReplay {
				t.Fatalf("shouldRetry = %v, want %v", got, tc.wantReplay)
			}
		})
	}
}

func TestResponsesQuotaReplayRetainsSafetyBoundaries(t *testing.T) {
	for name, change := range map[string]func(*gin.Context, *relaymodel.ErrorWithStatusCode){
		"already output": func(c *gin.Context, e *relaymodel.ErrorWithStatusCode) { _, _ = c.Writer.Write([]byte("partial")) },
		"local quota":    func(c *gin.Context, e *relaymodel.ErrorWithStatusCode) { e.Error.Code = "insufficient_user_balance" },
		"transport error": func(c *gin.Context, e *relaymodel.ErrorWithStatusCode) {
			e.Error.Type = "one_api_error"
			e.Error.Code = "do_request_failed"
			e.StatusCode = 502
		},
		"explicit channel": func(c *gin.Context, e *relaymodel.ErrorWithStatusCode) { c.Set(ctxkey.SpecificChannelId, "A") },
		"retry disabled": func(c *gin.Context, e *relaymodel.ErrorWithStatusCode) {
			p := routing.DefaultPolicy()
			p.RetryScope = routing.RetryScopeNone
			c.Set(ctxkey.ProviderRoutingPolicy, p)
		},
		"opaque routing token": func(c *gin.Context, e *relaymodel.ErrorWithStatusCode) {
			c.Request.Header.Set("x-codex-turn-state", "opaque-A")
		},
		"parent only": func(c *gin.Context, e *relaymodel.ErrorWithStatusCode) {
			c.Set(ctxkey.KeyRequestBody, []byte(`{"previous_response_id":"resp_A","input":[{"type":"function_call_output","call_id":"call_1","output":"VALUE"}]}`))
		},
		"canceled": func(c *gin.Context, e *relaymodel.ErrorWithStatusCode) {
			ctx, cancel := context.WithCancel(c.Request.Context())
			cancel()
			c.Request = c.Request.WithContext(ctx)
		},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := quotaReplayContext()
			change(c, err)
			if prepareResponsesQuotaReplay(c, err) || !isPinnedResponsesRequest(c) {
				t.Fatal("unsafe request detached")
			}
		})
	}
}
