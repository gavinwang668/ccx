package autopilot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ── ProfileChangeEvent（Phase 3A：画像变更事件，只读展示，不影响调度）──

// ProfileChangeEventType 画像变更事件类型。
// 与设计文档 §7.3 WebSocket 事件类型保持一致，不额外发明新类型。
type ProfileChangeEventType string

const (
	EventTypeProfileUpdated    ProfileChangeEventType = "profile_updated"
	EventTypeHealthChanged     ProfileChangeEventType = "health_changed"
	EventTypeDiscoveryComplete ProfileChangeEventType = "discovery_completed"
	EventTypeAutoMappingApply  ProfileChangeEventType = "auto_mapping_applied"
)

// ProfileChangeEvent 描述一次画像变更（健康状态、能力标签、发现完成、自动映射写回等）。
// 只包含已脱敏字段，不携带明文 API Key / Authorization / prompt。
type ProfileChangeEvent struct {
	EventUID    string                 `json:"eventUid"`
	ChannelUID  string                 `json:"channelUid"`
	ChannelKind string                 `json:"channelKind"`
	EndpointUID string                 `json:"endpointUid,omitempty"`
	MetricsKey  string                 `json:"metricsKey,omitempty"`
	EventType   ProfileChangeEventType `json:"eventType"`
	Summary     string                 `json:"summary"`
	OldValue    string                 `json:"oldValue,omitempty"`
	NewValue    string                 `json:"newValue,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
}

// GenerateChangeEventUID 生成画像变更事件唯一标识。
func GenerateChangeEventUID(endpointUID string, eventType ProfileChangeEventType, createdAt time.Time) string {
	h := fmt.Sprintf("%s|%s|%s", endpointUID, string(eventType), createdAt.Format(time.RFC3339Nano))
	sum := sha256.Sum256([]byte(h))
	return "pc_" + hex.EncodeToString(sum[:8])
}

// DetectProfileChanges 对比旧/新 KeyEndpointProfile，返回本轮触发的变更事件。
//
// 规则（设计 §10 Phase 3「运行时指标驱动画像实时更新」的最小只读子集）：
//   - old == nil（首次画像）不产出任何事件，避免新渠道刷屏。
//   - HealthState 变化 → 单独一条 health_changed 事件（运维最关心的信号，单独归类）。
//   - QualityTier/StabilityTier/SpeedTier/CostTier/四个能力标签/groupChanged 中任一变化，
//     合并成一条 profile_updated 事件，Summary 列出变化的维度名，不逐字段发多条事件。
//   - 全部不变 → 返回空 slice。
//
// 纯函数，不读写任何 store，不影响调度，便于表驱动单测。
func DetectProfileChanges(old, current *KeyEndpointProfile, groupChanged bool, now time.Time) []ProfileChangeEvent {
	if current == nil {
		return nil
	}
	if old == nil {
		return nil
	}

	var events []ProfileChangeEvent

	if old.HealthState != current.HealthState {
		ev := ProfileChangeEvent{
			ChannelUID:  current.ChannelUID,
			ChannelKind: current.ChannelKind,
			EndpointUID: current.EndpointUID,
			MetricsKey:  current.MetricsKey,
			EventType:   EventTypeHealthChanged,
			Summary:     fmt.Sprintf("%s → %s", old.HealthState, current.HealthState),
			OldValue:    string(old.HealthState),
			NewValue:    string(current.HealthState),
			CreatedAt:   now,
		}
		ev.EventUID = GenerateChangeEventUID(ev.EndpointUID, ev.EventType, now)
		events = append(events, ev)
	}

	var changedDims []string
	if old.QualityTier != current.QualityTier {
		changedDims = append(changedDims, fmt.Sprintf("quality: %s → %s", old.QualityTier, current.QualityTier))
	}
	if old.StabilityTier != current.StabilityTier {
		changedDims = append(changedDims, fmt.Sprintf("stability: %s → %s", old.StabilityTier, current.StabilityTier))
	}
	if old.SpeedTier != current.SpeedTier {
		changedDims = append(changedDims, fmt.Sprintf("speed: %s → %s", old.SpeedTier, current.SpeedTier))
	}
	if old.CostTier != current.CostTier {
		changedDims = append(changedDims, fmt.Sprintf("cost: %s → %s", old.CostTier, current.CostTier))
	}
	if old.SupportsVision != current.SupportsVision {
		changedDims = append(changedDims, fmt.Sprintf("vision: %v → %v", old.SupportsVision, current.SupportsVision))
	}
	if old.SupportsToolCalls != current.SupportsToolCalls {
		changedDims = append(changedDims, fmt.Sprintf("toolCalls: %v → %v", old.SupportsToolCalls, current.SupportsToolCalls))
	}
	if old.SupportsReasoning != current.SupportsReasoning {
		changedDims = append(changedDims, fmt.Sprintf("reasoning: %v → %v", old.SupportsReasoning, current.SupportsReasoning))
	}
	if old.SupportsLongCtx != current.SupportsLongCtx {
		changedDims = append(changedDims, fmt.Sprintf("longCtx: %v → %v", old.SupportsLongCtx, current.SupportsLongCtx))
	}
	if groupChanged {
		changedDims = append(changedDims, "modelList: changed")
	}

	if len(changedDims) > 0 {
		ev := ProfileChangeEvent{
			ChannelUID:  current.ChannelUID,
			ChannelKind: current.ChannelKind,
			EndpointUID: current.EndpointUID,
			MetricsKey:  current.MetricsKey,
			EventType:   EventTypeProfileUpdated,
			Summary:     strings.Join(changedDims, "; "),
			CreatedAt:   now,
		}
		ev.EventUID = GenerateChangeEventUID(ev.EndpointUID, ev.EventType, now)
		events = append(events, ev)
	}

	return events
}
