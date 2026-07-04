package vectors

import (
	"fmt"
	"strings"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/handlers/common"
	"github.com/BenedictKing/ccx/internal/scheduler"
	"github.com/gin-gonic/gin"
)

type embeddingCompatibilityKey struct {
	spaceID    string
	dimensions int
	normalized int
}

type embeddingCompatibilityCandidate struct {
	channel     scheduler.ChannelInfo
	channelName string
	actualModel string
	available   bool
	known       bool
	valid       bool
	key         embeddingCompatibilityKey
	reason      string
}

func newEmbeddingCompatibilityFilter(c *gin.Context, requestModel string, requestDimensions int) scheduler.CandidateFilterFunc {
	return func(
		channels []scheduler.ChannelInfo,
		upstreamFor func(scheduler.ChannelInfo) *config.UpstreamConfig,
		candidateAvailable func(scheduler.ChannelInfo, *config.UpstreamConfig) bool,
	) ([]scheduler.ChannelInfo, error) {
		candidates := make([]embeddingCompatibilityCandidate, 0, len(channels))
		anyKnown := false

		for _, ch := range channels {
			upstream := upstreamFor(ch)
			channelName := ch.Name
			if upstream != nil && strings.TrimSpace(upstream.Name) != "" {
				channelName = upstream.Name
			}
			candidate := embeddingCompatibilityCandidate{
				channel:     ch,
				channelName: channelName,
			}
			if candidateAvailable == nil {
				candidate.available = true
			} else {
				candidate.available = candidateAvailable(ch, upstream)
			}
			if upstream == nil {
				candidate.reason = "missing upstream config"
				candidates = append(candidates, candidate)
				continue
			}

			resolved := config.ResolveEmbeddingCapability(requestModel, upstream)
			candidate.actualModel = strings.TrimSpace(resolved.ActualModel)
			if !resolved.Known {
				candidates = append(candidates, candidate)
				continue
			}

			candidate.known = true
			key, ok, reason := embeddingCompatibilityKeyFor(resolved, requestDimensions)
			candidate.key = key
			candidate.valid = ok
			candidate.reason = reason
			if candidate.available {
				anyKnown = true
			}
			candidates = append(candidates, candidate)
		}

		if !anyKnown {
			return channels, nil
		}

		var anchor *embeddingCompatibilityCandidate
		for i := range candidates {
			if candidates[i].available && candidates[i].known && candidates[i].valid {
				anchor = &candidates[i]
				break
			}
		}
		if anchor == nil {
			return nil, fmt.Errorf("no Vectors channel has embedding compatibility metadata compatible with model %q", requestModel)
		}

		filtered := make([]scheduler.ChannelInfo, 0, len(channels))
		for _, candidate := range candidates {
			if !candidate.available {
				common.RequestLogf(c, "[Vectors-EmbeddingCompat] skip channel [%d] %s: channel is not currently selectable for embedding compatibility",
					candidate.channel.Index, candidate.channelName)
				continue
			}
			if !candidate.known {
				common.RequestLogf(c, "[Vectors-EmbeddingCompat] skip channel [%d] %s: no embedding capability for actual_model=%q while strict compatibility is active",
					candidate.channel.Index, candidate.channelName, candidate.actualModel)
				continue
			}
			if !candidate.valid {
				common.RequestLogf(c, "[Vectors-EmbeddingCompat] skip channel [%d] %s: %s",
					candidate.channel.Index, candidate.channelName, candidate.reason)
				continue
			}
			if candidate.key != anchor.key {
				common.RequestLogf(c, "[Vectors-EmbeddingCompat] skip channel [%d] %s: incompatible embedding space actual_model=%q space=%q dimensions=%d normalized=%d anchor_channel=%s anchor_space=%q anchor_dimensions=%d anchor_normalized=%d",
					candidate.channel.Index,
					candidate.channelName,
					candidate.actualModel,
					candidate.key.spaceID,
					candidate.key.dimensions,
					candidate.key.normalized,
					anchor.channelName,
					anchor.key.spaceID,
					anchor.key.dimensions,
					anchor.key.normalized,
				)
				continue
			}
			filtered = append(filtered, candidate.channel)
		}

		common.RequestLogf(c, "[Vectors-EmbeddingCompat] anchor channel [%d] %s: space=%q dimensions=%d normalized=%d kept=%d/%d",
			anchor.channel.Index, anchor.channelName, anchor.key.spaceID, anchor.key.dimensions, anchor.key.normalized, len(filtered), len(channels))
		return filtered, nil
	}
}

func embeddingCompatibilityKeyFor(resolved config.ResolvedEmbeddingCapability, requestDimensions int) (embeddingCompatibilityKey, bool, string) {
	actualModel := strings.TrimSpace(resolved.ActualModel)
	capability := resolved.Capability
	if capability.Dimensions < 0 {
		return embeddingCompatibilityKey{}, false, fmt.Sprintf("invalid default dimensions %d for actual_model=%q", capability.Dimensions, actualModel)
	}
	for _, dimension := range capability.SupportedDimensions {
		if dimension <= 0 {
			return embeddingCompatibilityKey{}, false, fmt.Sprintf("invalid supported dimensions %d for actual_model=%q", dimension, actualModel)
		}
	}
	if requestDimensions > 0 && !embeddingCapabilitySupportsDimensions(capability, requestDimensions) {
		return embeddingCompatibilityKey{}, false, fmt.Sprintf("requested dimensions %d are not supported by actual_model=%q", requestDimensions, actualModel)
	}

	spaceID := strings.TrimSpace(capability.EmbeddingSpaceID)
	if spaceID == "" {
		spaceID = actualModel
	}
	if spaceID == "" {
		return embeddingCompatibilityKey{}, false, "missing actual embedding model"
	}

	effectiveDimensions := capability.Dimensions
	if requestDimensions > 0 {
		effectiveDimensions = requestDimensions
	}

	return embeddingCompatibilityKey{
		spaceID:    spaceID,
		dimensions: effectiveDimensions,
		normalized: embeddingNormalizedState(capability.Normalized),
	}, true, ""
}

func embeddingCapabilitySupportsDimensions(capability config.EmbeddingCapability, requestDimensions int) bool {
	if requestDimensions <= 0 {
		return true
	}
	if capability.Dimensions == requestDimensions {
		return true
	}
	for _, dimension := range capability.SupportedDimensions {
		if dimension == requestDimensions {
			return true
		}
	}
	return false
}

func embeddingNormalizedState(normalized *bool) int {
	if normalized == nil {
		return -1
	}
	if *normalized {
		return 1
	}
	return 0
}
