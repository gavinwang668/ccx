package autopilot

import (
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/scheduler"
)

func TestBuildChannelEntryUsesRequestedChannelKind(t *testing.T) {
	store := newTestProfileStore(t)
	if store == nil {
		t.Skip("ProfileStore 初始化失败")
	}

	profiles := []*KeyEndpointProfile{
		{
			EndpointUID: "ep_messages", ChannelUID: "ch_shared", ChannelKind: "messages",
			MetricsKey: "metrics_messages", HealthState: HealthStateDead,
			QualityTier: QualityTierLow, StabilityTier: StabilityTierUnstable,
			SpeedTier: SpeedTierSlow, CostTier: CostTierExpensive,
		},
		{
			EndpointUID: "ep_responses", ChannelUID: "ch_shared", ChannelKind: "responses",
			MetricsKey: "metrics_responses", HealthState: HealthStateHealthy,
			QualityTier: QualityTierHigh, StabilityTier: StabilityTierStable,
			SpeedTier: SpeedTierFast, CostTier: CostTierCheap,
		},
	}
	for _, profile := range profiles {
		if err := store.Upsert(profile); err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}
	}

	router := NewSmartRouter(store, nil, nil, nil)
	entry := router.buildChannelEntry(
		scheduler.ChannelInfo{Index: 0, Name: "shared", Status: "active"},
		&config.UpstreamConfig{ChannelUID: "ch_shared", Name: "shared"},
		"responses",
	)

	if entry.ChannelKind != "responses" {
		t.Fatalf("ChannelKind = %q, want responses", entry.ChannelKind)
	}
	if entry.MetricsKey != "metrics_responses" {
		t.Fatalf("MetricsKey = %q, want metrics_responses", entry.MetricsKey)
	}
	if entry.HealthState != HealthStateHealthy {
		t.Fatalf("HealthState = %q, want healthy", entry.HealthState)
	}
	if entry.ScoringCandidate.QualityTier != QualityTierHigh {
		t.Fatalf("QualityTier = %q, want high", entry.ScoringCandidate.QualityTier)
	}
}
