// Package common 提供 handlers 模块的公共功能
package common

import (
	"bytes"
	"encoding/json"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/utils"
	"github.com/gin-gonic/gin"
)

// isImageContentType 判断 content block 是否为图片类型
func isImageContentType(t string) bool {
	return t == "image" || t == "image_url" || t == "input_image"
}

// isImagePart 判断 Gemini part 是否为图片类型
func isImagePart(part map[string]interface{}) bool {
	if _, ok := part["inlineData"]; ok {
		return true
	}
	if _, ok := part["fileData"]; ok {
		return true
	}
	// 兼容蛇形字段名
	if _, ok := part["inline_data"]; ok {
		return true
	}
	if _, ok := part["file_data"]; ok {
		return true
	}
	return false
}

// replaceImageBlock 将图片 content block 替换为文本占位符
func replaceImageBlock(block map[string]interface{}, isResponsesAPI bool) map[string]interface{} {
	if isResponsesAPI {
		return map[string]interface{}{
			"type": "input_text",
			"text": "[Image]",
		}
	}
	return map[string]interface{}{
		"type": "text",
		"text": "[Image]",
	}
}

// replaceImagePart 将 Gemini 图片 part 替换为文本占位符
func replaceImagePart() map[string]interface{} {
	return map[string]interface{}{
		"text": "[Image]",
	}
}

// stripImagesInContent 递归替换 content 数组中的图片为占位符。
// 返回替换数量。
func stripImagesInContent(content []interface{}, isResponsesAPI bool) int {
	replaced := 0
	for i, block := range content {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := blockMap["type"].(string)
		if isImageContentType(t) {
			content[i] = replaceImageBlock(blockMap, isResponsesAPI)
			replaced++
		} else if nested, ok := blockMap["content"]; ok {
			// 递归处理嵌套 content（如 tool_result.content）
			switch v := nested.(type) {
			case []interface{}:
				replaced += stripImagesInContent(v, isResponsesAPI)
			case string:
				// content 是纯字符串，不含图片
			}
		}
	}
	return replaced
}

// countUserTurnsInMessages 统计 messages 数组中 role=="user" 的消息数
func countUserTurnsInMessages(messages []interface{}) int {
	count := 0
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := msgMap["role"].(string); role == "user" {
			count++
		}
	}
	return count
}

// countUserTurnsInContents 统计 Gemini contents 数组中 role=="user" 的消息数
func countUserTurnsInContents(contents []interface{}) int {
	count := 0
	for _, c := range contents {
		cMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := cMap["role"].(string); role == "user" {
			count++
		}
	}
	return count
}

// countUserTurnsInInput 统计 Responses input 中 user 消息数
// input 可能包含 {type:"message", role:"user"} 或省略 type 的 {role:"user", content:[...]}
func countUserTurnsInInput(input []interface{}) int {
	count := 0
	for _, item := range input {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := itemMap["type"].(string)
		role, _ := itemMap["role"].(string)
		if role == "user" {
			// 明确 type:"message" 或无 type 字段（省略 type 的合法格式）都计数
			if t == "message" || t == "" {
				count++
			}
		}
	}
	return count
}

// StripHistoricalImages 替换历史轮次中的图片为文本占位符。
// 返回 (修改后的 bodyBytes, 是否修改)。
// turnLimit: 保留最近 N 个 user 消息轮次的图片（0=不限制）。
func StripHistoricalImages(bodyBytes []byte, turnLimit int) ([]byte, bool) {
	return StripHistoricalImagesWithContext(nil, bodyBytes, turnLimit, false, "")
}

// StripHistoricalImagesWithContext 替换历史轮次中的图片为文本占位符（带日志上下文）。
func StripHistoricalImagesWithContext(c *gin.Context, bodyBytes []byte, turnLimit int, enableLog bool, apiType string) ([]byte, bool) {
	if turnLimit <= 0 || len(bodyBytes) == 0 {
		return bodyBytes, false
	}

	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.UseNumber()

	var data map[string]interface{}
	if err := decoder.Decode(&data); err != nil {
		return bodyBytes, false
	}

	// 用最终 body 的准确值刷新 vision 缓存。
	// 覆盖所有退出路径（含 early return），防止多渠道 failover 时
	// 前一个渠道的替换把缓存污染为 false，后一个渠道复用 stale 缓存。
	finalBody := &bodyBytes
	defer func() {
		if c != nil {
			c.Set(visionDetectedContextKey, detectImageInBody(*finalBody))
		}
	}()

	totalReplaced := 0

	// Claude Messages / OpenAI Chat: messages[*].content[*]
	if messages, ok := data["messages"].([]interface{}); ok && len(messages) > 0 {
		totalUserTurns := countUserTurnsInMessages(messages)
		if totalUserTurns <= turnLimit {
			return bodyBytes, false
		}
		turnsToStrip := totalUserTurns - turnLimit

		userTurnIdx := 0
		for _, msg := range messages {
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := msgMap["role"].(string)
			if role == "user" {
				userTurnIdx++
			}

			if userTurnIdx > 0 && userTurnIdx <= turnsToStrip {
				if content, ok := msgMap["content"].([]interface{}); ok {
					totalReplaced += stripImagesInContent(content, false)
				}
			}
		}
	}

	// Responses API: input[*]
	if input, ok := data["input"].([]interface{}); ok && len(input) > 0 {
		totalUserTurns := countUserTurnsInInput(input)
		if totalUserTurns > turnLimit {
			turnsToStrip := totalUserTurns - turnLimit

			userTurnIdx := 0
			for idx := range input {
				itemMap, ok := input[idx].(map[string]interface{})
				if !ok {
					continue
				}
				t, _ := itemMap["type"].(string)
				role, _ := itemMap["role"].(string)
				if role == "user" && (t == "message" || t == "") {
					userTurnIdx++
				}

				if userTurnIdx > 0 && userTurnIdx <= turnsToStrip {
					// 替换顶层 item 中的图片
					if isImageContentType(t) {
						input[idx] = map[string]interface{}{
							"type": "input_text",
							"text": "[Image]",
						}
						totalReplaced++
					}
					// 替换 content 数组中的图片
					if content, ok := itemMap["content"].([]interface{}); ok {
						totalReplaced += stripImagesInContent(content, true)
					}
				}
			}
		}
	}

	// Gemini: contents[*].parts[*]
	if contents, ok := data["contents"].([]interface{}); ok && len(contents) > 0 {
		totalUserTurns := countUserTurnsInContents(contents)
		if totalUserTurns > turnLimit {
			turnsToStrip := totalUserTurns - turnLimit

			userTurnIdx := 0
			for _, c := range contents {
				cMap, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				role, _ := cMap["role"].(string)
				if role == "user" {
					userTurnIdx++
				}

				if userTurnIdx > 0 && userTurnIdx <= turnsToStrip {
					if parts, ok := cMap["parts"].([]interface{}); ok {
						for i, part := range parts {
							if partMap, ok := part.(map[string]interface{}); ok {
								if isImagePart(partMap) {
									parts[i] = replaceImagePart()
									totalReplaced++
								}
							}
						}
					}
				}
			}
		}
	}

	if totalReplaced == 0 {
		return bodyBytes, false
	}

	// 重新序列化
	newBytes, err := utils.MarshalJSONNoEscape(data)
	if err != nil {
		return bodyBytes, false
	}

	if enableLog && apiType != "" {
		RequestLogf(c, "[%s-Vision] 历史图片轮次限制生效: limit=%d, replaced=%d", apiType, turnLimit, totalReplaced)
	}

	*finalBody = newBytes
	return newBytes, true
}

// resolveHistoricalImageTurnLimit 解析渠道级历史图片轮次限制。
// 返回 0 表示该渠道不裁剪；大于 0 时归一到 2-10。
func resolveHistoricalImageTurnLimit(upstream *config.UpstreamConfig) int {
	if upstream != nil && upstream.HistoricalImageTurnLimit > 0 {
		return config.NormalizeChannelHistoricalImageTurnLimit(upstream.HistoricalImageTurnLimit)
	}
	return 0
}
