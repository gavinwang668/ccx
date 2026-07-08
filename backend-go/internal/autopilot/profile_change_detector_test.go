package autopilot

import (
	"strings"
	"testing"
	"time"
)

func TestDetectProfileChanges_NilOldProducesNoEvents(t *testing.T) {
	current := &KeyEndpointProfile{
		ChannelUID:  "ch-1",
		EndpointUID: "ep-1",
		HealthState: HealthStateHealthy,
	}
	events := DetectProfileChanges(nil, current, false, time.Now())
	if len(events) != 0 {
		t.Fatalf("old=nil 应不产出任何事件，got %d", len(events))
	}
}

func TestDetectProfileChanges_NilCurrentProducesNoEvents(t *testing.T) {
	old := &KeyEndpointProfile{ChannelUID: "ch-1", EndpointUID: "ep-1"}
	events := DetectProfileChanges(old, nil, false, time.Now())
	if len(events) != 0 {
		t.Fatalf("current=nil 应不产出任何事件，got %d", len(events))
	}
}

func TestDetectProfileChanges_NoChangeProducesNoEvents(t *testing.T) {
	p := &KeyEndpointProfile{
		ChannelUID:        "ch-1",
		ChannelKind:       "messages",
		EndpointUID:       "ep-1",
		HealthState:       HealthStateHealthy,
		QualityTier:       QualityTierHigh,
		StabilityTier:     StabilityTierStable,
		SpeedTier:         SpeedTierFast,
		CostTier:          CostTierCheap,
		SupportsVision:    true,
		SupportsToolCalls: true,
	}
	old := *p
	events := DetectProfileChanges(&old, p, false, time.Now())
	if len(events) != 0 {
		t.Fatalf("全部维度不变应返回空 slice，got %d: %+v", len(events), events)
	}
}

func TestDetectProfileChanges_HealthStateChange(t *testing.T) {
	now := time.Now()
	old := &KeyEndpointProfile{
		ChannelUID:  "ch-1",
		ChannelKind: "messages",
		EndpointUID: "ep-1",
		MetricsKey:  "mk-1",
		HealthState: HealthStateHealthy,
	}
	current := &KeyEndpointProfile{
		ChannelUID:  "ch-1",
		ChannelKind: "messages",
		EndpointUID: "ep-1",
		MetricsKey:  "mk-1",
		HealthState: HealthStateDegraded,
	}

	events := DetectProfileChanges(old, current, false, now)
	if len(events) != 1 {
		t.Fatalf("HealthState 变化应产出恰好 1 条事件，got %d", len(events))
	}
	ev := events[0]
	if ev.EventType != EventTypeHealthChanged {
		t.Errorf("EventType = %q, want %q", ev.EventType, EventTypeHealthChanged)
	}
	if ev.OldValue != string(HealthStateHealthy) || ev.NewValue != string(HealthStateDegraded) {
		t.Errorf("OldValue/NewValue = %q/%q, want healthy/degraded", ev.OldValue, ev.NewValue)
	}
	if ev.EventUID == "" {
		t.Error("EventUID 不应为空")
	}
	if !strings.HasPrefix(ev.EventUID, "pc_") {
		t.Errorf("EventUID 应以 pc_ 开头: %q", ev.EventUID)
	}
	if ev.ChannelUID != "ch-1" || ev.EndpointUID != "ep-1" || ev.MetricsKey != "mk-1" {
		t.Errorf("身份字段透传不正确: %+v", ev)
	}
}

func TestDetectProfileChanges_MultipleDimensionsMergeIntoOneEvent(t *testing.T) {
	now := time.Now()
	old := &KeyEndpointProfile{
		ChannelUID:        "ch-1",
		EndpointUID:       "ep-1",
		HealthState:       HealthStateHealthy,
		QualityTier:       QualityTierNormal,
		StabilityTier:     StabilityTierStable,
		SpeedTier:         SpeedTierFast,
		CostTier:          CostTierCheap,
		SupportsVision:    false,
		SupportsToolCalls: false,
		SupportsReasoning: false,
		SupportsLongCtx:   false,
	}
	current := &KeyEndpointProfile{
		ChannelUID:        "ch-1",
		EndpointUID:       "ep-1",
		HealthState:       HealthStateHealthy, // 不变
		QualityTier:       QualityTierHigh,    // 变
		StabilityTier:     StabilityTierStable,
		SpeedTier:         SpeedTierFast,
		CostTier:          CostTierCheap,
		SupportsVision:    true, // 变
		SupportsToolCalls: false,
		SupportsReasoning: false,
		SupportsLongCtx:   false,
	}

	events := DetectProfileChanges(old, current, false, now)
	if len(events) != 1 {
		t.Fatalf("多维度变化（health 不变）应合并成 1 条 profile_updated 事件，got %d: %+v", len(events), events)
	}
	ev := events[0]
	if ev.EventType != EventTypeProfileUpdated {
		t.Errorf("EventType = %q, want %q", ev.EventType, EventTypeProfileUpdated)
	}
	if !strings.Contains(ev.Summary, "quality:") {
		t.Errorf("Summary 应包含 quality 维度变化: %q", ev.Summary)
	}
	if !strings.Contains(ev.Summary, "vision:") {
		t.Errorf("Summary 应包含 vision 维度变化: %q", ev.Summary)
	}
	if strings.Contains(ev.Summary, "stability:") {
		t.Errorf("Summary 不应包含未变化的 stability 维度: %q", ev.Summary)
	}
}

func TestDetectProfileChanges_HealthAndOtherDimensionsBothChange(t *testing.T) {
	now := time.Now()
	old := &KeyEndpointProfile{
		ChannelUID:  "ch-1",
		EndpointUID: "ep-1",
		HealthState: HealthStateHealthy,
		QualityTier: QualityTierNormal,
	}
	current := &KeyEndpointProfile{
		ChannelUID:  "ch-1",
		EndpointUID: "ep-1",
		HealthState: HealthStateDead,
		QualityTier: QualityTierHigh,
	}

	events := DetectProfileChanges(old, current, false, now)
	if len(events) != 2 {
		t.Fatalf("health + 其他维度同时变化应产出 2 条独立事件，got %d: %+v", len(events), events)
	}

	var hasHealth, hasProfile bool
	for _, ev := range events {
		switch ev.EventType {
		case EventTypeHealthChanged:
			hasHealth = true
		case EventTypeProfileUpdated:
			hasProfile = true
		}
	}
	if !hasHealth || !hasProfile {
		t.Errorf("应同时包含 health_changed 和 profile_updated，got %+v", events)
	}
}

func TestDetectProfileChanges_GroupChangedAloneTriggersProfileUpdated(t *testing.T) {
	now := time.Now()
	old := &KeyEndpointProfile{ChannelUID: "ch-1", EndpointUID: "ep-1", HealthState: HealthStateHealthy}
	current := &KeyEndpointProfile{ChannelUID: "ch-1", EndpointUID: "ep-1", HealthState: HealthStateHealthy}

	events := DetectProfileChanges(old, current, true, now)
	if len(events) != 1 {
		t.Fatalf("仅 groupChanged=true 应产出 1 条 profile_updated 事件，got %d", len(events))
	}
	if events[0].EventType != EventTypeProfileUpdated {
		t.Errorf("EventType = %q, want %q", events[0].EventType, EventTypeProfileUpdated)
	}
	if !strings.Contains(events[0].Summary, "modelList") {
		t.Errorf("Summary 应提及 modelList 变化: %q", events[0].Summary)
	}
}

func TestGenerateChangeEventUID_DeterministicAndPrefixed(t *testing.T) {
	now := time.Now()
	uid1 := GenerateChangeEventUID("ep-1", EventTypeHealthChanged, now)
	uid2 := GenerateChangeEventUID("ep-1", EventTypeHealthChanged, now)
	if uid1 != uid2 {
		t.Errorf("相同输入应生成相同 UID: %q != %q", uid1, uid2)
	}
	if !strings.HasPrefix(uid1, "pc_") {
		t.Errorf("UID 应以 pc_ 开头: %q", uid1)
	}

	uid3 := GenerateChangeEventUID("ep-2", EventTypeHealthChanged, now)
	if uid1 == uid3 {
		t.Error("不同 endpointUID 应生成不同 UID")
	}
}
