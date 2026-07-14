package autopilot

import (
	"encoding/json"
	"io"
	"math"
	"strings"
)

func scoreProviderQualityOutput(text string, latencyMs int64) (ProviderQualityDimensions, ProviderQualityEvidence, float64) {
	evidence := ProviderQualityEvidence{ContentPresent: strings.TrimSpace(text) != ""}
	dimensions := ProviderQualityDimensions{Latency: providerQualityLatencyScore(latencyMs)}
	if !evidence.ContentPresent {
		return dimensions, evidence, 0
	}

	object, strict, parsed := parseProviderQualityCanary(text)
	evidence.StrictJSON = strict
	dimensions.Completeness = 0.25
	if parsed {
		dimensions.Completeness += 0.25
		dimensions.Format = 0.25
		if strict {
			dimensions.Format = 1
		}

		if providerQualityNumber(object["answer"], 323) {
			evidence.CorrectFields++
		}
		if _, ok := object["answer"].(json.Number); ok {
			evidence.RequiredFields++
		}
		if providerQualityNumber(object["sequence"], 13) {
			evidence.CorrectFields++
		}
		if _, ok := object["sequence"].(json.Number); ok {
			evidence.RequiredFields++
		}
		if checksum, ok := object["checksum"].(string); ok {
			evidence.RequiredFields++
			if checksum == "ABC" {
				evidence.CorrectFields++
			}
		}
		dimensions.Completeness += 0.5 * float64(evidence.RequiredFields) / 3
		dimensions.Semantic = float64(evidence.CorrectFields) / 3
	}

	dimensions.Completeness = roundProviderQuality(dimensions.Completeness)
	dimensions.Semantic = roundProviderQuality(dimensions.Semantic)
	dimensions.Format = roundProviderQuality(dimensions.Format)
	dimensions.Latency = roundProviderQuality(dimensions.Latency)
	score := 0.40*dimensions.Completeness +
		0.30*dimensions.Semantic +
		0.15*dimensions.Format +
		0.15*dimensions.Latency
	return dimensions, evidence, roundProviderQuality(score)
}

func parseProviderQualityCanary(text string) (map[string]any, bool, bool) {
	trimmed := strings.TrimSpace(text)
	if object, ok := decodeProviderQualityObject(trimmed); ok {
		return object, true, true
	}

	recovered := trimmed
	if strings.HasPrefix(recovered, "```") {
		lines := strings.Split(recovered, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			recovered = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	if object, ok := decodeProviderQualityObject(recovered); ok {
		return object, false, true
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		if object, ok := decodeProviderQualityObject(trimmed[start : end+1]); ok {
			return object, false, true
		}
	}
	return nil, false, false
}

func decodeProviderQualityObject(text string) (map[string]any, bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil {
		return nil, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, false
	}
	return object, true
}

func providerQualityNumber(value any, expected int64) bool {
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	actual, err := number.Int64()
	return err == nil && actual == expected
}

func providerQualityLatencyScore(latencyMs int64) float64 {
	switch {
	case latencyMs <= 2_000:
		return 1
	case latencyMs <= 10_000:
		return 1 - 0.4*float64(latencyMs-2_000)/8_000
	case latencyMs <= 30_000:
		return 0.6 - 0.4*float64(latencyMs-10_000)/20_000
	case latencyMs <= 60_000:
		return 0.2 - 0.2*float64(latencyMs-30_000)/30_000
	default:
		return 0
	}
}

func aggregateProviderQualitySamples(samples []ProviderQualitySampleResult, successCount int) (float64, float64) {
	if len(samples) == 0 || successCount == 0 {
		return 0, 0
	}
	var total float64
	for _, sample := range samples {
		total += sample.Score
	}
	score := total / float64(len(samples))
	baseConfidence := 0.5 + 0.1*float64(successCount)
	reliability := float64(successCount) / float64(len(samples))
	confidence := math.Min(0.8, baseConfidence) * reliability
	return roundProviderQuality(score), roundProviderQuality(confidence)
}

func roundProviderQuality(value float64) float64 {
	return math.Round(value*1000) / 1000
}
