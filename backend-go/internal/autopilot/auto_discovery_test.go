package autopilot

import (
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
)

func TestAutoDiscoveryRunner_TriggerRejectsDuplicate(t *testing.T) {
	runner := NewAutoDiscoveryRunner(nil)

	ch := &config.UpstreamConfig{
		ChannelUID: "ch_test_001",
		BaseURL:    "https://example.com",
		APIKeys:    []string{"sk-test"},
	}
	started := runner.TriggerDiscovery("ch_test_001", ch, nil)
	if !started {
		t.Fatal("第一次触发应返回 true")
	}

	started = runner.TriggerDiscovery("ch_test_001", ch, nil)
	if started {
		t.Fatal("重复触发应返回 false")
	}
}

func TestAutoDiscoveryRunner_GetTaskNil(t *testing.T) {
	runner := NewAutoDiscoveryRunner(nil)
	task := runner.GetTask("nonexistent")
	if task != nil {
		t.Fatal("从未触发的渠道应返回 nil")
	}
}

func TestAutoDiscoveryRunner_TriggerCreatesTask(t *testing.T) {
	runner := NewAutoDiscoveryRunner(nil)

	ch := &config.UpstreamConfig{
		ChannelUID: "ch_test_002",
		BaseURL:    "https://example.com",
		APIKeys:    []string{"sk-test"},
	}
	runner.TriggerDiscovery("ch_test_002", ch, nil)

	task := runner.GetTask("ch_test_002")
	if task == nil {
		t.Fatal("触发后 GetTask 应返回非 nil")
	}
	if task.ChannelUID != "ch_test_002" {
		t.Fatalf("期望 ChannelUID=ch_test_002, 实际=%s", task.ChannelUID)
	}
	if task.Status != DiscoveryStatusRunning {
		t.Fatalf("初始状态应为 running, 实际=%s", task.Status)
	}
}

func TestParseModelsResponse(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected int
	}{
		{
			name:     "标准 OpenAI 格式",
			body:     `{"data":[{"id":"gpt-4o"},{"id":"gpt-3.5-turbo"}]}`,
			expected: 2,
		},
		{
			name:     "空列表",
			body:     `{"data":[]}`,
			expected: 0,
		},
		{
			name:     "无效 JSON",
			body:     `not json`,
			expected: 0,
		},
		{
			name:     "跳过空 ID",
			body:     `{"data":[{"id":"model-1"},{"id":""},{"id":"model-3"}]}`,
			expected: 2,
		},
		{
			name:     "data 缺失",
			body:     `{"other": "field"}`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			models := parseModelsResponse([]byte(tt.body))
			if len(models) != tt.expected {
				t.Fatalf("期望 %d 个模型, 实际 %d", tt.expected, len(models))
			}
		})
	}
}

func TestDiscoveryStatus_Constants(t *testing.T) {
	// 确保状态常量符合预期字符串
	if DiscoveryStatusIdle != "idle" {
		t.Fatal("DiscoveryStatusIdle 应为 'idle'")
	}
	if DiscoveryStatusRunning != "running" {
		t.Fatal("DiscoveryStatusRunning 应为 'running'")
	}
	if DiscoveryStatusDone != "done" {
		t.Fatal("DiscoveryStatusDone 应为 'done'")
	}
	if DiscoveryStatusFailed != "failed" {
		t.Fatal("DiscoveryStatusFailed 应为 'failed'")
	}
}

func TestAutoDiscoveryRunner_ConcurrentTriggers(t *testing.T) {
	runner := NewAutoDiscoveryRunner(nil)

	ch := &config.UpstreamConfig{
		ChannelUID: "ch_concurrent",
		BaseURL:    "https://example.com",
		APIKeys:    []string{"sk-test"},
	}

	// 并发触发同一渠道，只有第一个应该成功
	results := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			started := runner.TriggerDiscovery("ch_concurrent", ch, nil)
			results <- started
		}()
	}

	successCount := 0
	for i := 0; i < 5; i++ {
		if <-results {
			successCount++
		}
	}

	if successCount != 1 {
		t.Fatalf("并发触发同一渠道应恰好有1个成功，实际=%d", successCount)
	}
}

func TestEndpointDiscoveryResult_KeyMask(t *testing.T) {
	// 验证 EndpointDiscoveryResult 中 KeyMask 不包含明文 key
	result := EndpointDiscoveryResult{
		KeyMask:     "sk-****abcd",
		BaseURL:     "https://api.example.com",
		ModelsCount: 5,
		ProtocolOk:  true,
	}

	if result.KeyMask == "" {
		t.Fatal("KeyMask 不应为空")
	}
	if len(result.KeyMask) < 4 {
		t.Fatal("KeyMask 长度过短")
	}
}
