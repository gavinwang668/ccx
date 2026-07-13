package scheduler

import (
	"context"
	"testing"
)

type candidateFilterContextKey struct{}

func TestCandidateFilterProviderReceivesRequestContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), candidateFilterContextKey{}, "request-profile")
	s := &ChannelScheduler{}

	called := false
	s.SetCandidateFilterProvider(func(gotCtx context.Context, kind ChannelKind, model string) CandidateFilterFunc {
		called = true
		if gotCtx.Value(candidateFilterContextKey{}) != "request-profile" {
			t.Fatal("provider did not receive request context")
		}
		if kind != ChannelKindResponses || model != "claude-sonnet-5" {
			t.Fatalf("unexpected provider arguments: kind=%q model=%q", kind, model)
		}
		return nil
	})

	if filter := s.buildSmartFilterFromProvider(ctx, ChannelKindResponses, "claude-sonnet-5"); filter != nil {
		t.Fatal("buildSmartFilterFromProvider() returned non-nil filter")
	}
	if !called {
		t.Fatal("candidate filter provider was not called")
	}
}
