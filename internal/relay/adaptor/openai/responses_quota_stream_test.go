package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponsesQuotaStreamOnlyRetriesBeforeOutput(t *testing.T) {
	const prelude = "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_a\",\"output\":[]}}\n\n"
	const quota = "data: {\"type\":\"error\",\"error\":{\"type\":\"insufficient_quota\",\"code\":\"credit_balance_exhausted\",\"param\":null},\"sequence_number\":2}\n\n"
	const failed = "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"project_spend_limit_exceeded\"},\"output\":[]}}\n\n"
	const delta = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"visible\"}\n\n"
	const tool = "data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\"}}\n\n"
	const slow = "data: {\"type\":\"error\",\"code\":\"slow_down\",\"message\":\"Try again shortly.\"}\n\n"
	for _, tc := range []struct {
		name, stream, wantCode string
	}{
		{name: "published event", stream: quota, wantCode: "credit_balance_exhausted"},
		{name: "after empty prelude", stream: prelude + quota, wantCode: "credit_balance_exhausted"},
		{name: "failed response envelope", stream: prelude + failed, wantCode: "project_spend_limit_exceeded"},
		{name: "after text", stream: prelude + delta + quota},
		{name: "after tool", stream: prelude + tool + quota},
		{name: "temporary throttle", stream: prelude + slow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newOpenAITestContext()
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.stream))}
			err, _ := StreamResponsesHandler(c, resp, "gpt-5.4", 1)
			if tc.wantCode == "" {
				if err != nil || !strings.HasPrefix(recorder.Body.String(), tc.stream) {
					t.Fatalf("visible/ordinary stream changed: error=%+v body=%q", err, recorder.Body.String())
				}
				return
			}
			if err == nil || err.StatusCode != http.StatusTooManyRequests || err.Code != tc.wantCode || c.Writer.Written() {
				t.Fatalf("quota should be recovered before writing: error=%+v written=%t", err, c.Writer.Written())
			}
		})
	}
}
