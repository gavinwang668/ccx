package common

import "strings"

const claudeNoVisibleOutputRetryPrompt = "[Your previous response had no visible output. Please continue and produce a user-visible response.]"

// IsClaudeNoVisibleOutputRetryPrompt 检测 Claude Code 注入的无可见输出重试提示。
func IsClaudeNoVisibleOutputRetryPrompt(text string) bool {
	return strings.TrimSpace(text) == claudeNoVisibleOutputRetryPrompt
}
