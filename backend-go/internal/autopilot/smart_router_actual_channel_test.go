package autopilot

import (
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/scheduler"
)

func TestCandidateFilterForWithActualCorrelatesOwnTrace(t *testing.T) {
	cfg := baseTestConfig()
	cfg.AutopilotRouting = config.AutopilotRoutingConfig{RoutingMode: "shadow"}
	cfgManager, cleanup := createTestConfigManager(t, cfg)
	defer cleanup()

	traceStore, err := NewTraceStoreWithDB(nil)
	if err != nil {
		t.Fatalf("NewTraceStoreWithDB() error = %v", err)
	}
	router := NewSmartRouter(nil, nil, traceStore, cfgManager)
	processed := cfgManager.GetConfig()

	channels := make([]scheduler.ChannelInfo, 0, len(processed.Upstream))
	for i, upstream := range processed.Upstream {
		channels = append(channels, scheduler.ChannelInfo{
			Index:    i,
			Name:     upstream.Name,
			Priority: upstream.Priority,
			Status:   upstream.Status,
		})
	}
	upstreamFor := func(ch scheduler.ChannelInfo) *config.UpstreamConfig {
		upstream := processed.Upstream[ch.Index]
		return &upstream
	}
	available := func(ch scheduler.ChannelInfo, upstream *config.UpstreamConfig) bool {
		return ch.Status == "active" && upstream != nil && len(upstream.APIKeys) > 0
	}

	firstProfile := testProfile()
	firstProfile.Model = "first-model"
	firstFilter, updateFirst := router.CandidateFilterForWithActual(firstProfile)
	secondProfile := testProfile()
	secondProfile.Model = "second-model"
	secondFilter, updateSecond := router.CandidateFilterForWithActual(secondProfile)
	if firstFilter == nil || updateFirst == nil || secondFilter == nil || updateSecond == nil {
		t.Fatal("shadow mode should return filters and observers")
	}

	if _, err := firstFilter(channels, upstreamFor, available); err != nil {
		t.Fatalf("first filter error = %v", err)
	}
	if _, err := secondFilter(channels, upstreamFor, available); err != nil {
		t.Fatalf("second filter error = %v", err)
	}

	firstActual := processed.Upstream[1].ChannelUID
	secondActual := processed.Upstream[0].ChannelUID
	updateFirst(firstActual)
	updateSecond(secondActual)

	byModel := make(map[string]*RoutingDecisionTrace)
	for _, trace := range traceStore.ListRecent(10) {
		byModel[trace.RequestedModel] = trace
	}
	assertActual := func(model, want string) {
		t.Helper()
		trace := byModel[model]
		if trace == nil {
			t.Fatalf("missing trace for model %q", model)
		}
		if trace.ActualChannelUID != want {
			t.Fatalf("model %q actual channel = %q, want %q", model, trace.ActualChannelUID, want)
		}
		if trace.Match != (trace.ShadowChannelUID == want) {
			t.Fatalf("model %q match = %v, shadow=%q actual=%q", model, trace.Match, trace.ShadowChannelUID, want)
		}
	}
	assertActual(firstProfile.Model, firstActual)
	assertActual(secondProfile.Model, secondActual)
}
