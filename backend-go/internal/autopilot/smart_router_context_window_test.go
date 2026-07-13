package autopilot

import (
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/scheduler"
)

func TestBuildChannelEntryResolvesContextWindow(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		upstream   config.UpstreamConfig
		global     map[string]config.UpstreamModelCapability
		wantTokens int
	}{
		{
			name:  "渠道模型能力覆盖优先",
			model: "alias-model",
			upstream: config.UpstreamConfig{
				ModelMapping: map[string]string{"alias-model": "actual-model"},
				ModelCapabilities: map[string]config.UpstreamModelCapability{
					"actual-model": {ContextWindowTokens: 4096},
				},
			},
			global: map[string]config.UpstreamModelCapability{
				"actual-model": {ContextWindowTokens: 8192},
			},
			wantTokens: 4096,
		},
		{
			name:  "映射后命中全局能力",
			model: "alias-model",
			upstream: config.UpstreamConfig{
				ModelMapping: map[string]string{"alias-model": "actual-model"},
			},
			global: map[string]config.UpstreamModelCapability{
				"actual-model": {ContextWindowTokens: 8192},
			},
			wantTokens: 8192,
		},
		{
			name:       "命中内置模型注册表",
			model:      "mimo-v2.5-pro",
			wantTokens: 1_048_576,
		},
		{
			name:       "未知模型保持 fail-open",
			model:      "unknown-model-without-capability",
			wantTokens: 0,
		},
	}

	router := NewSmartRouter(nil, nil, nil, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := tt.upstream
			upstream.ChannelUID = "ch_context"
			entry := router.buildChannelEntry(
				scheduler.ChannelInfo{Index: 0, Name: "context", Status: "active"},
				&upstream,
				"messages",
				tt.model,
				tt.global,
			)
			if entry.ContextWindowTokens != tt.wantTokens {
				t.Fatalf("ContextWindowTokens = %d, want %d", entry.ContextWindowTokens, tt.wantTokens)
			}
		})
	}
}

func TestResolvedContextWindowFeedsAutoHardConstraint(t *testing.T) {
	router := NewSmartRouter(nil, nil, nil, nil)
	upstream := &config.UpstreamConfig{
		ChannelUID: "ch_short",
		ModelCapabilities: map[string]config.UpstreamModelCapability{
			"short-model": {ContextWindowTokens: 4096},
		},
	}
	entry := router.buildChannelEntry(
		scheduler.ChannelInfo{Index: 0, Name: "short", Status: "active"},
		upstream,
		"messages",
		"short-model",
		nil,
	)

	reasons := autoHardConstraintReasons(&RequestProfile{ContextNeed: 4097}, &entry)
	if len(reasons) != 1 || reasons[0] != "上下文窗口不满足" {
		t.Fatalf("autoHardConstraintReasons() = %v, want [上下文窗口不满足]", reasons)
	}
}
