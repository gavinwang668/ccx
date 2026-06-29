package common

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
)

// countImageBlocks 递归统计 JSON 字节中图片块/图片 part 的数量
func countImageBlocks(t *testing.T, body []byte) int {
	t.Helper()
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	count := 0
	var walkContent func(content []interface{})
	walkContent = func(content []interface{}) {
		for _, block := range content {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}
			t, _ := blockMap["type"].(string)
			if isImageContentType(t) {
				count++
			}
			if nested, ok := blockMap["content"].([]interface{}); ok {
				walkContent(nested)
			}
		}
	}
	if messages, ok := data["messages"].([]interface{}); ok {
		for _, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				if content, ok := msgMap["content"].([]interface{}); ok {
					walkContent(content)
				}
			}
		}
	}
	if input, ok := data["input"].([]interface{}); ok {
		for _, item := range input {
			if itemMap, ok := item.(map[string]interface{}); ok {
				it, _ := itemMap["type"].(string)
				if isImageContentType(it) {
					count++
				}
				if content, ok := itemMap["content"].([]interface{}); ok {
					walkContent(content)
				}
			}
		}
	}
	if contents, ok := data["contents"].([]interface{}); ok {
		for _, c := range contents {
			if cMap, ok := c.(map[string]interface{}); ok {
				if parts, ok := cMap["parts"].([]interface{}); ok {
					for _, part := range parts {
						if partMap, ok := part.(map[string]interface{}); ok {
							if isImagePart(partMap) {
								count++
							}
						}
					}
				}
			}
		}
	}
	return count
}

func TestStripHistoricalImages_ClaudeMessages_LimitExceeded(t *testing.T) {
	// 3 轮 user 消息，limit=1，前 2 轮图片应被替换，最后一轮保留
	body := `{"messages":[
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img1"}}]},
		{"role":"assistant","content":[{"type":"text","text":"ok"}]},
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img2"}}]},
		{"role":"assistant","content":[{"type":"text","text":"ok"}]},
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img3"}}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 1)
	if !modified {
		t.Fatal("expected modified=true")
	}
	if got := countImageBlocks(t, result); got != 1 {
		t.Errorf("remaining images = %d, want 1", got)
	}
	// 确认替换为 [Image] 占位符
	if !strings.Contains(string(result), "[Image]") {
		t.Error("expected [Image] placeholder in result")
	}
	// 确认 img1/img2 不再泄露
	if strings.Contains(string(result), "img1") || strings.Contains(string(result), "img2") {
		t.Error("historical image data leaked")
	}
	// 确认最后一轮 img3 保留
	if !strings.Contains(string(result), "img3") {
		t.Error("latest turn image should be preserved")
	}
}

func TestStripHistoricalImages_ClaudeMessages_LimitNotReached(t *testing.T) {
	// 2 轮 user，limit=3，无需替换
	body := `{"messages":[
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img1"}}]},
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img2"}}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 3)
	if modified {
		t.Error("expected modified=false when totalTurns <= limit")
	}
	if got := countImageBlocks(t, result); got != 2 {
		t.Errorf("images = %d, want 2 (unchanged)", got)
	}
}

func TestStripHistoricalImages_ClaudeMessages_NestedToolResult(t *testing.T) {
	// tool_result 内嵌套图片应递归替换
	body := `{"messages":[
		{"role":"user","content":[{"type":"tool_result","content":[{"type":"image","source":{"type":"base64","data":"nested1"}}]}]},
		{"role":"user","content":[{"type":"text","text":"q"}]},
		{"role":"user","content":[{"type":"text","text":"q"}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 2)
	if !modified {
		t.Fatal("expected modified=true")
	}
	if got := countImageBlocks(t, result); got != 0 {
		t.Errorf("remaining images = %d, want 0", got)
	}
	if strings.Contains(string(result), "nested1") {
		t.Error("nested image data leaked")
	}
}

func TestStripHistoricalImages_OpenAIChat_ImageURL(t *testing.T) {
	body := `{"messages":[
		{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://x.com/a.png"}}]},
		{"role":"user","content":[{"type":"text","text":"q"}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 1)
	if !modified {
		t.Fatal("expected modified=true")
	}
	if got := countImageBlocks(t, result); got != 0 {
		t.Errorf("remaining images = %d, want 0", got)
	}
	if strings.Contains(string(result), "x.com/a.png") {
		t.Error("image url leaked")
	}
}

func TestStripHistoricalImages_ResponsesAPI(t *testing.T) {
	// input_image 替换为 input_text，保留同轮文本
	body := `{"input":[
		{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,xxx"},{"type":"input_text","text":"keep me"}]},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"latest"}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 1)
	if !modified {
		t.Fatal("expected modified=true")
	}
	if got := countImageBlocks(t, result); got != 0 {
		t.Errorf("remaining images = %d, want 0", got)
	}
	if !strings.Contains(string(result), "input_text") {
		t.Error("expected input_text placeholder for Responses API")
	}
	if !strings.Contains(string(result), "keep me") {
		t.Error("same-turn text should be preserved")
	}
	if strings.Contains(string(result), "base64,xxx") {
		t.Error("image data leaked")
	}
}

func TestStripHistoricalImages_ResponsesAPI_OmittedType(t *testing.T) {
	// Responses 合法的省略 type 的格式：{role:"user", content:[...]}（无 type:"message"）
	body := `{"input":[
		{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,old"},{"type":"input_text","text":"q1"}]},
		{"role":"user","content":[{"type":"input_text","text":"latest"}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 1)
	if !modified {
		t.Fatal("expected modified=true for omitted-type Responses input")
	}
	if got := countImageBlocks(t, result); got != 0 {
		t.Errorf("remaining images = %d, want 0", got)
	}
	if strings.Contains(string(result), "base64,old") {
		t.Error("historical image data leaked (omitted-type format)")
	}
	if !strings.Contains(string(result), "q1") {
		t.Error("same-turn text should be preserved")
	}
}

func TestStripHistoricalImages_ResponsesAPI_TopLevelInputImage(t *testing.T) {
	// input 顶层直接是 input_image 的情况
	body := `{"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"q1"}]},
		{"type":"input_image","image_url":"data:image/png;base64,top"},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"q2"}]}
	]}`

	// 2 user 轮，limit=1，第一轮（含顶层 input_image）应替换
	result, modified := StripHistoricalImages([]byte(body), 1)
	if !modified {
		t.Fatal("expected modified=true")
	}
	if strings.Contains(string(result), "base64,top") {
		t.Error("top-level input_image data leaked")
	}
}

func TestStripHistoricalImages_Gemini(t *testing.T) {
	body := `{"contents":[
		{"role":"user","parts":[{"inlineData":{"mimeType":"image/png","data":"geminiimg1"}}]},
		{"role":"model","parts":[{"text":"ok"}]},
		{"role":"user","parts":[{"text":"latest"}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 1)
	if !modified {
		t.Fatal("expected modified=true")
	}
	if got := countImageBlocks(t, result); got != 0 {
		t.Errorf("remaining image parts = %d, want 0", got)
	}
	if strings.Contains(string(result), "geminiimg1") {
		t.Error("gemini inline image data leaked")
	}
}

func TestStripHistoricalImages_Gemini_SnakeCase(t *testing.T) {
	// 兼容蛇形字段 inline_data / file_data
	body := `{"contents":[
		{"role":"user","parts":[{"inline_data":{"mime_type":"image/png","data":"snake1"}}]},
		{"role":"user","parts":[{"file_data":{"file_uri":"gs://x"}}]},
		{"role":"user","parts":[{"text":"latest"}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 1)
	if !modified {
		t.Fatal("expected modified=true")
	}
	if got := countImageBlocks(t, result); got != 0 {
		t.Errorf("remaining image parts = %d, want 0", got)
	}
	if strings.Contains(string(result), "snake1") || strings.Contains(string(result), "gs://x") {
		t.Error("snake_case image data leaked")
	}
}

func TestStripHistoricalImages_NoImages(t *testing.T) {
	body := `{"messages":[
		{"role":"user","content":[{"type":"text","text":"a"}]},
		{"role":"user","content":[{"type":"text","text":"b"}]},
		{"role":"user","content":[{"type":"text","text":"c"}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 1)
	if modified {
		t.Error("expected modified=false when no images present")
	}
	if string(result) != body {
		t.Error("body should be unchanged")
	}
}

func TestStripHistoricalImages_ZeroLimit(t *testing.T) {
	body := `{"messages":[
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img1"}}]},
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img2"}}]}
	]}`

	result, modified := StripHistoricalImages([]byte(body), 0)
	if modified {
		t.Error("expected modified=false when limit=0")
	}
	if got := countImageBlocks(t, result); got != 2 {
		t.Errorf("images = %d, want 2 (unchanged)", got)
	}
}

func TestStripHistoricalImages_AssistantHistoricalImage(t *testing.T) {
	// assistant 历史消息中的图片按最近 user 轮次归属替换
	body := `{"messages":[
		{"role":"user","content":[{"type":"text","text":"q1"}]},
		{"role":"assistant","content":[{"type":"image","source":{"type":"base64","data":"assistimg"}}]},
		{"role":"user","content":[{"type":"text","text":"q2"}]},
		{"role":"user","content":[{"type":"text","text":"latest"}]}
	]}`

	// 3 user 轮，limit=2 → strip 第 1 轮。assistant 图片在第 1 个 user 之后，归属第 1 轮，应被替换。
	result, modified := StripHistoricalImages([]byte(body), 2)
	if !modified {
		t.Fatal("expected modified=true")
	}
	if strings.Contains(string(result), "assistimg") {
		t.Error("assistant historical image should be stripped")
	}
}

func TestStripHistoricalImages_VisionCacheInvalidation(t *testing.T) {
	// 替换后 HasImageContent 应返回 false（缓存被清理后重新检测）
	body := `{"messages":[
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img1"}}]},
		{"role":"user","content":[{"type":"text","text":"latest"}]}
	]}`

	c := newTestContext()
	// 先触发一次检测，填充缓存为 true
	if !HasImageContent(c, []byte(body)) {
		t.Fatal("original body should be detected as having image")
	}

	result, modified := StripHistoricalImagesWithContext(c, []byte(body), 1, false, "Messages")
	if !modified {
		t.Fatal("expected modified=true")
	}
	// 替换后无图片，缓存应已清理，重新检测返回 false
	if HasImageContent(c, result) {
		t.Error("after stripping, HasImageContent should return false (cache must be invalidated)")
	}
}

func TestStripHistoricalImages_VisionCacheStillTrueWhenRecentImageRemains(t *testing.T) {
	// 关键回归：替换历史图片后，最近 N 轮仍有图片时，
	// vision 缓存必须为 true，否则 vision fallback 会被错误跳过。
	body := `{"messages":[
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"old"}}]},
		{"role":"user","content":[{"type":"text","text":"mid"}]},
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"recent"}}]}
	]}`

	c := newTestContext()
	if !HasImageContent(c, []byte(body)) {
		t.Fatal("original body should be detected as having image")
	}

	// 3 user 轮，limit=2 → strip 第 1 轮（old 图）；最近 2 轮中的 recent 图保留
	result, modified := StripHistoricalImagesWithContext(c, []byte(body), 2, false, "Messages")
	if !modified {
		t.Fatal("expected modified=true")
	}
	// 替换后 body 仍含 recent 图，缓存必须为 true
	if !HasImageContent(c, result) {
		t.Error("after stripping, HasImageContent should remain true when recent turns still contain images")
	}
	if strings.Contains(string(result), "\"old\"") {
		t.Error("historical image should be stripped")
	}
	if !strings.Contains(string(result), "recent") {
		t.Error("recent image should be preserved")
	}
}

func TestStripHistoricalImages_MultiChannelNoPollution(t *testing.T) {
	// 同一原始请求按不同 limit 替换，原始 body 不被污染
	original := `{"messages":[
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img1"}}]},
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img2"}}]},
		{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"img3"}}]}
	]}`
	originalBytes := []byte(original)

	// 渠道 A：limit=1
	resA, _ := StripHistoricalImages(originalBytes, 1)
	// 渠道 B：limit=2
	resB, _ := StripHistoricalImages(originalBytes, 2)

	if countImageBlocks(t, resA) != 1 {
		t.Errorf("channel A remaining = %d, want 1", countImageBlocks(t, resA))
	}
	if countImageBlocks(t, resB) != 2 {
		t.Errorf("channel B remaining = %d, want 2", countImageBlocks(t, resB))
	}
	// 原始 body 不被污染
	if countImageBlocks(t, originalBytes) != 3 {
		t.Errorf("original body polluted: images = %d, want 3", countImageBlocks(t, originalBytes))
	}
}

func TestResolveHistoricalImageTurnLimit_DefaultUnlimited(t *testing.T) {
	if got := resolveHistoricalImageTurnLimit(nil); got != 0 {
		t.Fatalf("default limit = %d, want 0 (unlimited)", got)
	}

	upstream := &config.UpstreamConfig{HistoricalImageTurnLimit: 0}
	if got := resolveHistoricalImageTurnLimit(upstream); got != 0 {
		t.Fatalf("disabled channel limit = %d, want 0 (unlimited)", got)
	}

	upstream.HistoricalImageTurnLimit = 1
	if got := resolveHistoricalImageTurnLimit(upstream); got != config.HistoricalImageTurnLimitMin {
		t.Fatalf("channel limit = %d, want min %d", got, config.HistoricalImageTurnLimitMin)
	}

	upstream.HistoricalImageTurnLimit = 5
	if got := resolveHistoricalImageTurnLimit(upstream); got != 5 {
		t.Fatalf("channel limit = %d, want 5", got)
	}

	upstream.HistoricalImageTurnLimit = 11
	if got := resolveHistoricalImageTurnLimit(upstream); got != config.HistoricalImageTurnLimitMax {
		t.Fatalf("channel limit = %d, want max %d", got, config.HistoricalImageTurnLimitMax)
	}
}
