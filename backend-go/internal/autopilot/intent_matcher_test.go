package autopilot

import (
	"testing"
	"time"
)

// ── IntentMatchContext 构造辅助 ──

func testMatchCtx() *IntentMatchContext {
	return &IntentMatchContext{
		ChannelKind: "messages",
		Model:       "fable-5",
		TaskClass:   TaskClassWorker,
		AgentRole:   "subagent",
	}
}

// ── model_trial 匹配测试 ──

func TestMatchIntent_ModelTrial_Hit(t *testing.T) {
	ctx := testMatchCtx()
	intents := []*ManualRoutingIntent{
		{
			IntentUID:      "mi_test_001",
			IntentType:     IntentTypeModelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_target",
			Model:          "fable-5",
			TrafficPercent: 100,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result == nil {
		t.Fatal("model_trial 应命中")
	}
	if result.Intent.IntentUID != "mi_test_001" {
		t.Errorf("IntentUID = %q, want %q", result.Intent.IntentUID, "mi_test_001")
	}
	if result.ChannelUID != "ch_target" {
		t.Errorf("ChannelUID = %q, want %q", result.ChannelUID, "ch_target")
	}
	if len(result.Reasons) == 0 {
		t.Error("Reasons 不应为空")
	}
}

func TestMatchIntent_ModelTrial_MissModel(t *testing.T) {
	ctx := testMatchCtx()
	ctx.Model = "claude-sonnet-4" // 不匹配
	intents := []*ManualRoutingIntent{
		{
			IntentUID:   "mi_test_002",
			IntentType:  IntentTypeModelTrial,
			ChannelKind: "messages",
			ChannelUID:  "ch_target",
			Model:       "fable-5",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
			Status:      IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result != nil {
		t.Error("模型不匹配时 model_trial 不应命中")
	}
}

func TestMatchIntent_ModelTrial_MissChannelKind(t *testing.T) {
	ctx := testMatchCtx()
	intents := []*ManualRoutingIntent{
		{
			IntentUID:   "mi_test_003",
			IntentType:  IntentTypeModelTrial,
			ChannelKind: "chat", // 不匹配
			ChannelUID:  "ch_target",
			Model:       "fable-5",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
			Status:      IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result != nil {
		t.Error("ChannelKind 不匹配时不应命中")
	}
}

func TestMatchIntent_ModelTrial_EmptyModel(t *testing.T) {
	ctx := testMatchCtx()
	intents := []*ManualRoutingIntent{
		{
			IntentUID:   "mi_test_004",
			IntentType:  IntentTypeModelTrial,
			ChannelKind: "messages",
			ChannelUID:  "ch_target",
			Model:       "", // 空模型
			ExpiresAt:   time.Now().Add(1 * time.Hour),
			Status:      IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result != nil {
		t.Error("意图模型为空时 model_trial 不应命中")
	}
}

// ── session_pin 匹配测试 ──

func TestMatchIntent_SessionPin_Hit(t *testing.T) {
	ctx := testMatchCtx()
	ctx.SessionID = "sess_abc123"
	ctx.Model = "" // session_pin 不需要 model
	intents := []*ManualRoutingIntent{
		{
			IntentUID:   "mi_sess_001",
			IntentType:  IntentTypeSessionPin,
			ChannelKind: "messages",
			ChannelUID:  "ch_debug",
			SessionID:   "sess_abc123",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
			Status:      IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result == nil {
		t.Fatal("session_pin 应命中")
	}
	if result.ChannelUID != "ch_debug" {
		t.Errorf("ChannelUID = %q, want %q", result.ChannelUID, "ch_debug")
	}
}

func TestMatchIntent_SessionPin_MissSessionID(t *testing.T) {
	ctx := testMatchCtx()
	ctx.SessionID = "sess_different"
	ctx.Model = ""
	intents := []*ManualRoutingIntent{
		{
			IntentUID:   "mi_sess_002",
			IntentType:  IntentTypeSessionPin,
			ChannelKind: "messages",
			ChannelUID:  "ch_debug",
			SessionID:   "sess_abc123",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
			Status:      IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result != nil {
		t.Error("SessionID 不匹配时 session_pin 不应命中")
	}
}

func TestMatchIntent_SessionPin_EmptySessionID(t *testing.T) {
	ctx := testMatchCtx()
	ctx.SessionID = "" // 无 session ID
	ctx.Model = ""
	intents := []*ManualRoutingIntent{
		{
			IntentUID:   "mi_sess_003",
			IntentType:  IntentTypeSessionPin,
			ChannelKind: "messages",
			ChannelUID:  "ch_debug",
			SessionID:   "sess_abc123",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
			Status:      IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result != nil {
		t.Error("请求无 SessionID 时 session_pin 不应命中")
	}
}

// ── TrafficPercent 确定性测试 ──

func TestMatchIntent_TrafficPercent_Deterministic(t *testing.T) {
	ctx := testMatchCtx()
	ctx.PromptHash = "a1b2c3d4e5f67890"
	intents := []*ManualRoutingIntent{
		{
			IntentUID:      "mi_tp_001",
			IntentType:     IntentTypeModelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_target",
			Model:          "fable-5",
			TrafficPercent: 50,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
	}

	// 多次调用应返回一致结果（确定性）
	firstResult := MatchIntent(ctx, intents)
	for i := 0; i < 10; i++ {
		result := MatchIntent(ctx, intents)
		if (result == nil) != (firstResult == nil) {
			t.Fatalf("第 %d 次调用结果不一致: first=%v current=%v", i, firstResult != nil, result != nil)
		}
		if result != nil && result.Intent.IntentUID != firstResult.Intent.IntentUID {
			t.Errorf("第 %d 次调用 IntentUID 不一致", i)
		}
	}
}

func TestMatchIntent_TrafficPercent_AlwaysMatch(t *testing.T) {
	ctx := testMatchCtx()
	ctx.PromptHash = "abc123"
	intents := []*ManualRoutingIntent{
		{
			IntentUID:      "mi_tp_002",
			IntentType:     IntentTypeModelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_target",
			Model:          "fable-5",
			TrafficPercent: 0, // 0 = 匹配全部
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result == nil {
		t.Error("TrafficPercent=0 应匹配全部流量")
	}
}

func TestMatchIntent_TrafficPercent_NoHashInput(t *testing.T) {
	ctx := testMatchCtx()
	ctx.PromptHash = ""
	ctx.SessionID = "" // 无可哈希输入
	intents := []*ManualRoutingIntent{
		{
			IntentUID:      "mi_tp_003",
			IntentType:     IntentTypeModelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_target",
			Model:          "fable-5",
			TrafficPercent: 50, // 需要哈希但无输入
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result != nil {
		t.Error("无可哈希输入且 TrafficPercent<100 时不应匹配")
	}
}

func TestDeterministicBucket_Consistency(t *testing.T) {
	// 同一输入应始终映射到同一桶
	input := "test_hash_input_abc123"
	expected := deterministicBucket(input)
	for i := 0; i < 100; i++ {
		got := deterministicBucket(input)
		if got != expected {
			t.Fatalf("第 %d 次 deterministicBucket(%q) = %d, want %d", i, input, got, expected)
		}
	}

	// 不同输入应能产生不同桶（大数统计）
	buckets := make(map[uint32]bool)
	for i := 0; i < 1000; i++ {
		b := deterministicBucket(string(rune('a'+i%26)) + string(rune('0'+i/26%10)))
		buckets[b] = true
	}
	if len(buckets) < 10 {
		t.Errorf("deterministicBucket 分布过于集中: 只产生 %d 个不同桶", len(buckets))
	}
}

// ── supervisor 保护测试 ──

func TestMatchIntent_SupervisorProtection_TaskClassFilter(t *testing.T) {
	// 意图限制 TaskClasses=[worker]，请求是 supervisor → 不匹配
	ctx := testMatchCtx()
	ctx.TaskClass = TaskClassSupervisor
	ctx.Model = "fable-5"
	intents := []*ManualRoutingIntent{
		{
			IntentUID:      "mi_sup_001",
			IntentType:     IntentTypeModelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_target",
			Model:          "fable-5",
			TaskClasses:    []TaskClass{TaskClassWorker, TaskClassLightweight},
			TrafficPercent: 100,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result != nil {
		t.Error("supervisor 请求不应匹配限制 TaskClasses=[worker,lightweight] 的意图")
	}
}

func TestMatchIntent_SupervisorProtection_ExplicitSupervisor(t *testing.T) {
	// 意图显式包含 supervisor → 允许匹配
	ctx := testMatchCtx()
	ctx.TaskClass = TaskClassSupervisor
	ctx.Model = "fable-5"
	intents := []*ManualRoutingIntent{
		{
			IntentUID:      "mi_sup_002",
			IntentType:     IntentTypeModelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_target",
			Model:          "fable-5",
			TaskClasses:    []TaskClass{TaskClassSupervisor},
			TrafficPercent: 100,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result == nil {
		t.Error("意图显式包含 supervisor 时应允许匹配")
	}
}

func TestIntentExplicitlyTargetsSupervisor(t *testing.T) {
	tests := []struct {
		name     string
		intent   *ManualRoutingIntent
		expected bool
	}{
		{
			name:     "显式包含 supervisor",
			intent:   &ManualRoutingIntent{TaskClasses: []TaskClass{TaskClassSupervisor, TaskClassWorker}},
			expected: true,
		},
		{
			name:     "不包含 supervisor",
			intent:   &ManualRoutingIntent{TaskClasses: []TaskClass{TaskClassWorker, TaskClassLightweight}},
			expected: false,
		},
		{
			name:     "空 TaskClasses",
			intent:   &ManualRoutingIntent{TaskClasses: nil},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intentExplicitlyTargetsSupervisor(tt.intent)
			if got != tt.expected {
				t.Errorf("intentExplicitlyTargetsSupervisor() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ── 多意图排序测试 ──

func TestMatchIntent_MultipleIntents_SpecificitySort(t *testing.T) {
	ctx := testMatchCtx()
	ctx.SessionID = "sess_abc"
	ctx.PromptHash = "hash123"
	intents := []*ManualRoutingIntent{
		{
			// channel_trial: specificity 1 (channel_trial)
			IntentUID:      "mi_channel_001",
			IntentType:     IntentTypeChannelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_general",
			TaskClasses:    []TaskClass{TaskClassWorker},
			TrafficPercent: 100,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
		{
			// model_trial + task_class + agent_role: specificity 3
			IntentUID:      "mi_model_001",
			IntentType:     IntentTypeModelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_specific",
			Model:          "fable-5",
			TaskClasses:    []TaskClass{TaskClassWorker},
			AgentRoles:     []string{"subagent"},
			TrafficPercent: 100,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result == nil {
		t.Fatal("应命中最具体的意图")
	}
	if result.Intent.IntentUID != "mi_model_001" {
		t.Errorf("应选择 specificity 最高的意图，实际: %s", result.Intent.IntentUID)
	}
}

// ── nil/空输入测试 ──

func TestMatchIntent_NilContext(t *testing.T) {
	result := MatchIntent(nil, []*ManualRoutingIntent{{}})
	if result != nil {
		t.Error("nil context 应返回 nil")
	}
}

func TestMatchIntent_EmptyIntents(t *testing.T) {
	result := MatchIntent(testMatchCtx(), nil)
	if result != nil {
		t.Error("nil intents 应返回 nil")
	}

	result = MatchIntent(testMatchCtx(), []*ManualRoutingIntent{})
	if result != nil {
		t.Error("空 intents 应返回 nil")
	}
}

func TestMatchIntent_NilIntentInList(t *testing.T) {
	result := MatchIntent(testMatchCtx(), []*ManualRoutingIntent{nil})
	if result != nil {
		t.Error("nil intent 在列表中应被跳过")
	}
}

// ── channel_trial / endpoint_trial 测试 ──

func TestMatchIntent_ChannelTrial_Hit(t *testing.T) {
	ctx := testMatchCtx()
	ctx.Model = "" // channel_trial 不需要 model
	intents := []*ManualRoutingIntent{
		{
			IntentUID:      "mi_ch_001",
			IntentType:     IntentTypeChannelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_trial",
			TaskClasses:    []TaskClass{TaskClassWorker},
			TrafficPercent: 100,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result == nil {
		t.Fatal("channel_trial 应命中")
	}
	if result.ChannelUID != "ch_trial" {
		t.Errorf("ChannelUID = %q, want %q", result.ChannelUID, "ch_trial")
	}
}

func TestMatchIntent_AgentRole_Filter(t *testing.T) {
	ctx := testMatchCtx()
	ctx.AgentRole = "main" // 不匹配
	ctx.Model = ""
	intents := []*ManualRoutingIntent{
		{
			IntentUID:      "mi_ar_001",
			IntentType:     IntentTypeChannelTrial,
			ChannelKind:    "messages",
			ChannelUID:     "ch_trial",
			AgentRoles:     []string{"subagent"},
			TrafficPercent: 100,
			ExpiresAt:      time.Now().Add(1 * time.Hour),
			Status:         IntentStatusActive,
		},
	}

	result := MatchIntent(ctx, intents)
	if result != nil {
		t.Error("AgentRole 不匹配时不应命中")
	}
}

// ── containsTaskClass / containsString 测试 ──

func TestContainsTaskClass(t *testing.T) {
	tests := []struct {
		name     string
		list     []TaskClass
		target   TaskClass
		expected bool
	}{
		{"命中", []TaskClass{TaskClassWorker, TaskClassSupervisor}, TaskClassWorker, true},
		{"未命中", []TaskClass{TaskClassWorker}, TaskClassSupervisor, false},
		{"空列表", nil, TaskClassWorker, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsTaskClass(tt.list, tt.target)
			if got != tt.expected {
				t.Errorf("containsTaskClass() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		list     []string
		target   string
		expected bool
	}{
		{"命中", []string{"main", "subagent"}, "main", true},
		{"未命中", []string{"subagent"}, "main", false},
		{"空列表", nil, "main", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsString(tt.list, tt.target)
			if got != tt.expected {
				t.Errorf("containsString() = %v, want %v", got, tt.expected)
			}
		})
	}
}
