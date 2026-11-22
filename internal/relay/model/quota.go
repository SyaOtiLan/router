package model

import (
	"net/http"
	"strings"
)

func ResponsesPoolQuotaError() ErrorWithStatusCode {
	return ErrorWithStatusCode{
		StatusCode: http.StatusTooManyRequests,
		Error: Error{
			Type:    "usage_limit_reached",
			Code:    "usage_limit_reached",
			Message: "当前分组可用上游额度不足，请稍后再试",
		},
	}
}

// IsQuotaExhausted identifies explicit provider quota signals. Free-form
// messages are not sufficient evidence that a whole pool is exhausted.
func (e Error) IsQuotaExhausted() bool {
	switch strings.ToLower(strings.TrimSpace(e.Type)) {
	case "insufficient_quota", "usage_limit_reached":
		return true
	}
	code, _ := e.Code.(string)
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "insufficient_quota", "insufficient_user_quota", "billing_hard_limit_reached",
		"credit_balance_exhausted", "organization_spend_limit_exceeded", "project_spend_limit_exceeded",
		"organization_usage_limit_exceeded", "usage_limit_reached":
		return true
	}
	return false
}
