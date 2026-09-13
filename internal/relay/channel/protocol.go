package channel

import (
	"strconv"
	"strings"
)

var protocolNameToType map[string]int

func init() {
	protocolNameToType = make(map[string]int, len(ChannelProtocolNames))
	for id, raw := range ChannelProtocolNames {
		name := strings.TrimSpace(strings.ToLower(raw))
		if id <= 0 || id >= Dummy || name == "" {
			continue
		}
		protocolNameToType[name] = id
	}
	// Keep the historical doubao alias for volcengine channels.
	protocolNameToType["doubao"] = VolcEngine
}

func NormalizeProtocolName(raw string) string {
	name := strings.TrimSpace(strings.ToLower(raw))
	if name == "" {
		return ""
	}
	if normalizedID, ok := protocolNameToType[name]; ok {
		return ProtocolByType(normalizedID)
	}
	if numericID, err := strconv.Atoi(name); err == nil {
		return ProtocolByType(numericID)
	}
	return name
}

func ProtocolByType(channelProtocol int) string {
	if channelProtocol > 0 && channelProtocol < len(ChannelProtocolNames) {
		name := strings.TrimSpace(strings.ToLower(ChannelProtocolNames[channelProtocol]))
		if name != "" {
			return name
		}
	}
	return "openai"
}

func TypeByProtocol(protocol string) int {
	name := NormalizeProtocolName(protocol)
	if name == "" {
		return OpenAI
	}
	if id, ok := protocolNameToType[name]; ok {
		return id
	}
	return OpenAI
}

func BaseURLByProtocol(protocol string) string {
	channelProtocol := TypeByProtocol(protocol)
	if channelProtocol <= 0 || channelProtocol >= len(ChannelBaseURLs) {
		return ""
	}
	return strings.TrimSpace(ChannelBaseURLs[channelProtocol])
}
