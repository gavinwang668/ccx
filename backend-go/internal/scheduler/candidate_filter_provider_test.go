package scheduler

import (
	"context"
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
)

type candidateFilterContextKey struct{}

func TestCandidateFilterProviderReceivesRequestContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), candidateFilterContextKey{}, "request-profile")
	s := &ChannelScheduler{}

	called := false
	s.SetCandidateFilterProvider(func(gotCtx context.Context, kind ChannelKind, model string) (CandidateFilterFunc, CandidateSelectionObserver) {
		called = true
		if gotCtx.Value(candidateFilterContextKey{}) != "request-profile" {
			t.Fatal("provider did not receive request context")
		}
		if kind != ChannelKindResponses || model != "claude-sonnet-5" {
			t.Fatalf("unexpected provider arguments: kind=%q model=%q", kind, model)
		}
		return nil, nil
	})

	filter, observer := s.buildSmartFilterFromProvider(ctx, ChannelKindResponses, "claude-sonnet-5")
	if filter != nil || observer != nil {
		t.Fatal("buildSmartFilterFromProvider() returned non-nil filter")
	}
	if !called {
		t.Fatal("candidate filter provider was not called")
	}
}

func TestCandidateSelectionObserverReceivesActualChannelUID(t *testing.T) {
	s, cleanup := createTestScheduler(t, config.Config{
		Upstream: []config.UpstreamConfig{{
			Name:       "actual",
			ChannelUID: "ch_actual",
			BaseURL:    "https://actual.example.com",
			APIKeys:    []string{"sk-actual"},
			Status:     "active",
		}},
	})
	defer cleanup()

	observed := make([]string, 0, 1)
	s.SetCandidateFilterProvider(func(context.Context, ChannelKind, string) (CandidateFilterFunc, CandidateSelectionObserver) {
		filter := func(
			channels []ChannelInfo,
			_ func(ChannelInfo) *config.UpstreamConfig,
			_ func(ChannelInfo, *config.UpstreamConfig) bool,
		) ([]ChannelInfo, error) {
			return channels, nil
		}
		return filter, func(actualChannelUID string) {
			observed = append(observed, actualChannelUID)
		}
	})

	if _, err := s.SelectChannelWithOptions(context.Background(), SelectionOptions{Kind: ChannelKindMessages}); err != nil {
		t.Fatalf("SelectChannelWithOptions() error = %v", err)
	}
	if len(observed) != 1 || observed[0] != "ch_actual" {
		t.Fatalf("observer calls = %v, want [ch_actual]", observed)
	}

	observed = observed[:0]
	if _, err := s.SelectChannelWithOptions(context.Background(), SelectionOptions{Kind: ChannelKindMessages, DryRun: true}); err != nil {
		t.Fatalf("dry-run SelectChannelWithOptions() error = %v", err)
	}
	if len(observed) != 0 {
		t.Fatalf("dry-run unexpectedly called observer: %v", observed)
	}
}
