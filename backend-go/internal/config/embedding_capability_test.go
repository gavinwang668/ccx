package config

import "testing"

func TestResolveEmbeddingCapabilityUsesMappedActualModelPattern(t *testing.T) {
	normalized := true
	upstream := &UpstreamConfig{
		ModelMapping: map[string]string{
			"embed-public": "Vendor-Embedding-3-Small",
		},
		EmbeddingCapabilities: map[string]EmbeddingCapability{
			"vendor-embedding-3-*": {
				EmbeddingSpaceID:    "shared-space",
				Dimensions:          1536,
				SupportedDimensions: []int{512, 1536},
				Normalized:          &normalized,
			},
		},
	}

	resolved := ResolveEmbeddingCapability("embed-public", upstream)
	if !resolved.Known {
		t.Fatal("expected embedding capability to resolve")
	}
	if resolved.ActualModel != "Vendor-Embedding-3-Small" {
		t.Fatalf("ActualModel = %q, want mapped model", resolved.ActualModel)
	}
	if resolved.MatchedPattern != "vendor-embedding-3-*" {
		t.Fatalf("MatchedPattern = %q, want wildcard pattern", resolved.MatchedPattern)
	}
	if resolved.Capability.EmbeddingSpaceID != "shared-space" || resolved.Capability.Dimensions != 1536 {
		t.Fatalf("unexpected capability: %+v", resolved.Capability)
	}
	if resolved.Capability.Normalized == nil || !*resolved.Capability.Normalized {
		t.Fatalf("expected normalized=true, got %+v", resolved.Capability.Normalized)
	}
}
