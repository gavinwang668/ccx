package config

import "strings"

type ResolvedEmbeddingCapability struct {
	Capability     EmbeddingCapability
	RequestModel   string
	ActualModel    string
	MatchedPattern string
	Known          bool
}

func ResolveEmbeddingCapability(requestModel string, upstream *UpstreamConfig) ResolvedEmbeddingCapability {
	actualModel := strings.TrimSpace(requestModel)
	if upstream == nil {
		return ResolvedEmbeddingCapability{RequestModel: requestModel, ActualModel: actualModel}
	}

	actualModel = RedirectModel(requestModel, upstream)
	if capability, pattern, ok := resolvePatternValueFold(actualModel, upstream.EmbeddingCapabilities); ok {
		return ResolvedEmbeddingCapability{
			Capability:     capability,
			RequestModel:   requestModel,
			ActualModel:    actualModel,
			MatchedPattern: pattern,
			Known:          true,
		}
	}

	return ResolvedEmbeddingCapability{RequestModel: requestModel, ActualModel: actualModel}
}
