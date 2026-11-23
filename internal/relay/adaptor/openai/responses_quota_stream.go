package openai

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yeying-community/router/internal/relay/model"
)

const maxResponsesQuotaPrefixBytes = 64 * 1024

// Hold only the start of a Responses stream. Once an event can be observed by
// the client, the normal relay owns the stream and cannot replay it.
func readResponsesQuotaPrefix(scanner *bufio.Scanner) ([]string, *model.ErrorWithStatusCode) {
	var lines []string
	var data strings.Builder
	var eventName string
	size := 0
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		size += len(line) + 1
		if size > maxResponsesQuotaPrefixBytes {
			return lines, nil
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data.WriteString(strings.TrimPrefix(line, "data:"))
			data.WriteByte('\n')
		case line == "":
			if data.Len() != 0 {
				pending, quota := classifyResponsesQuotaEvent(eventName, data.String())
				if !pending {
					return lines, quota
				}
			}
			eventName = ""
			data.Reset()
		default:
			// Let upstream heartbeats and other SSE fields reach the client.
			return lines, nil
		}
	}
	return lines, nil
}

func classifyResponsesQuotaEvent(eventName, data string) (pending bool, quota *model.ErrorWithStatusCode) {
	var event struct {
		Type     string       `json:"type"`
		Code     any          `json:"code"`
		Message  string       `json:"message"`
		Error    *model.Error `json:"error"`
		Response struct {
			Error  *model.Error      `json:"error"`
			Output []json.RawMessage `json:"output"`
		} `json:"response"`
	}
	if json.Unmarshal([]byte(data), &event) != nil ||
		(eventName != "" && event.Type != "" && eventName != event.Type) || len(event.Response.Output) != 0 {
		return false, nil
	}
	if event.Type == "" {
		event.Type = eventName
	}
	var upstream *model.Error
	switch event.Type {
	case "response.created", "response.in_progress":
		return event.Error == nil && event.Response.Error == nil, nil
	case "error":
		upstream = event.Error
		if upstream == nil {
			upstream = &model.Error{Code: event.Code, Message: event.Message}
		}
	case "response.failed":
		upstream = event.Response.Error
	}
	if upstream == nil || !upstream.IsQuotaExhausted() {
		return false, nil
	}
	return false, &model.ErrorWithStatusCode{StatusCode: http.StatusTooManyRequests, Error: *upstream}
}
