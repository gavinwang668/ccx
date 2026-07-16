package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	claudeCodeProbeVersion       = "2.1.209"
	claudeCodeProbeUserAgent     = "claude-cli/" + claudeCodeProbeVersion + " (external, cli)"
	claudeCodeProbeBetaHeader    = "claude-code-20250219,adaptive-thinking-2026-01-28,prompt-caching-scope-2026-01-05,effort-2025-11-24"
	claudeCodeProbeBillingHeader = "x-anthropic-billing-header: cc_version=" + claudeCodeProbeVersion + ".2f9; cc_entrypoint=cli;"
	claudeCodeProbeIdentity      = "You are Claude Code, Anthropic's official CLI for Claude."
)

var (
	claudeCodeProbeDeviceID    = uuid.NewString()
	claudeCodeProbeAccountUUID = uuid.NewString()
)

type claudeCodeProbeUserID struct {
	DeviceID    string `json:"device_id"`
	AccountUUID string `json:"account_uuid"`
	SessionID   string `json:"session_id"`
}

func newClaudeCodeProbeMetadata() (map[string]string, string) {
	sessionID := uuid.NewString()
	userID, _ := json.Marshal(claudeCodeProbeUserID{
		DeviceID:    claudeCodeProbeDeviceID,
		AccountUUID: claudeCodeProbeAccountUUID,
		SessionID:   sessionID,
	})
	return map[string]string{"user_id": string(userID)}, sessionID
}

func newClaudeCodeProbeBillingBlock() map[string]interface{} {
	return map[string]interface{}{"type": "text", "text": claudeCodeProbeBillingHeader}
}

func newClaudeCodeProbeIdentityBlock() map[string]interface{} {
	return map[string]interface{}{
		"type":          "text",
		"text":          claudeCodeProbeIdentity,
		"cache_control": map[string]string{"type": "ephemeral"},
	}
}

// ensureClaudeCodeProbeBody 为所有 Messages 探针补齐 Claude Code 的 system 指纹与
// metadata.user_id JSON 字符串，并返回与请求体一致的会话 ID。
func ensureClaudeCodeProbeBody(body []byte) ([]byte, string) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, uuid.NewString()
	}

	changed := ensureClaudeCodeProbeSystem(payload)
	sessionID := claudeCodeProbeSessionID(payload)
	if sessionID == "" {
		metadata, generatedSessionID := newClaudeCodeProbeMetadata()
		payload["metadata"] = metadata
		sessionID = generatedSessionID
		changed = true
	}
	if !changed {
		return body, sessionID
	}

	updated, err := json.Marshal(payload)
	if err != nil {
		return body, sessionID
	}
	return updated, sessionID
}

func claudeCodeProbeSessionID(payload map[string]interface{}) string {
	metadata, ok := payload["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	userID, ok := metadata["user_id"].(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return ""
	}
	var parsed claudeCodeProbeUserID
	if err := json.Unmarshal([]byte(userID), &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.SessionID)
}

func ensureClaudeCodeProbeSystem(payload map[string]interface{}) bool {
	switch system := payload["system"].(type) {
	case []interface{}:
		var billingBlock interface{}
		var identityBlock interface{}
		billingCount := 0
		identityCount := 0
		rest := make([]interface{}, 0, len(system))
		for _, raw := range system {
			text := claudeCodeProbeSystemBlockText(raw)
			switch {
			case isClaudeCodeBillingBlock(text):
				billingCount++
				if billingBlock == nil {
					billingBlock = raw
				}
			case isClaudeCodeIdentityBlock(text):
				identityCount++
				if identityBlock == nil {
					identityBlock = raw
				}
			default:
				rest = append(rest, raw)
			}
		}
		if billingCount == 1 && identityCount == 1 && len(system) >= 2 &&
			isClaudeCodeBillingBlock(claudeCodeProbeSystemBlockText(system[0])) &&
			isClaudeCodeIdentityBlock(claudeCodeProbeSystemBlockText(system[1])) {
			return false
		}
		if billingBlock == nil {
			billingBlock = newClaudeCodeProbeBillingBlock()
		}
		if identityBlock == nil {
			identityBlock = newClaudeCodeProbeIdentityBlock()
		}
		payload["system"] = append([]interface{}{billingBlock, identityBlock}, rest...)
		return true
	case string:
		hasBilling := isClaudeCodeBillingBlock(system)
		hasIdentity := strings.Contains(system, claudeCodeProbeIdentity)
		if hasBilling && hasIdentity {
			return false
		}
		switch {
		case hasBilling:
			payload["system"] = system + "\n\n" + claudeCodeProbeIdentity
		case hasIdentity:
			payload["system"] = claudeCodeProbeBillingHeader + "\n\n" + system
		default:
			parts := []string{claudeCodeProbeBillingHeader, claudeCodeProbeIdentity}
			if strings.TrimSpace(system) != "" {
				parts = append(parts, system)
			}
			payload["system"] = strings.Join(parts, "\n\n")
		}
		return true
	}

	payload["system"] = []interface{}{
		newClaudeCodeProbeBillingBlock(),
		newClaudeCodeProbeIdentityBlock(),
	}
	return true
}

func isClaudeCodeBillingBlock(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "x-anthropic-billing-header") && strings.Contains(trimmed, "cc_entrypoint=")
}

func isClaudeCodeIdentityBlock(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), claudeCodeProbeIdentity)
}

func claudeCodeProbeSystemBlockText(raw interface{}) string {
	block, ok := raw.(map[string]interface{})
	if !ok {
		return ""
	}
	text, _ := block["text"].(string)
	return text
}

func applyClaudeCodeProbeHeaders(headers http.Header, sessionID string) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = uuid.NewString()
	}
	headers.Set("Accept", "application/json")
	headers.Set("anthropic-version", "2023-06-01")
	headers.Set("anthropic-beta", claudeCodeProbeBetaHeader)
	headers.Set("anthropic-dangerous-direct-browser-access", "true")
	headers.Set("User-Agent", claudeCodeProbeUserAgent)
	headers.Set("X-App", "cli")
	headers.Set("X-Claude-Code-Session-Id", sessionID)
}
