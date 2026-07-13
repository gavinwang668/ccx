package autopilot

// RequestProfileFeatures 是协议层提取出的脱敏请求特征。
// 这里只接收结构化元数据，不接收或持久化消息正文。
type RequestProfileFeatures struct {
	Model              string
	ChannelKind        string
	Operation          string
	AgentRole          string
	AgentType          string
	HasImage           bool
	EstTokens          int
	ContextNeed        int
	VisionNeed         bool
	ImageGenNeed       bool
	EmbeddingNeed      bool
	ToolUseNeed        bool
	ReasoningNeed      bool
	EmbeddingDimension int
	SessionID          string
	PromptHash         string
}

// BuildRequestProfile 将协议无关特征收敛为 SmartRouter 使用的请求画像。
// 未知字段保持保守零值；图片请求始终要求 vision，实际上下文需求默认取输入估算。
func BuildRequestProfile(features RequestProfileFeatures) RequestProfile {
	contextNeed := features.ContextNeed
	if contextNeed <= 0 {
		contextNeed = features.EstTokens
	}

	qualityNeed := QualityTierLow
	if features.Model != "" {
		family := InferModelFamily(features.Model, "")
		qualityNeed = ModelProfileQualityTierFromFamily(family, features.Model)
	}

	profile := RequestProfile{
		Model:              features.Model,
		ChannelKind:        features.ChannelKind,
		Operation:          features.Operation,
		AgentRole:          features.AgentRole,
		AgentType:          features.AgentType,
		HasImage:           features.HasImage,
		EstTokens:          features.EstTokens,
		QualityNeed:        qualityNeed,
		ContextNeed:        contextNeed,
		VisionNeed:         features.VisionNeed || features.HasImage,
		ImageGenNeed:       features.ImageGenNeed,
		EmbeddingNeed:      features.EmbeddingNeed,
		ToolUseNeed:        features.ToolUseNeed,
		ReasoningNeed:      features.ReasoningNeed,
		EmbeddingDimension: features.EmbeddingDimension,
		SessionID:          features.SessionID,
		PromptHash:         features.PromptHash,
	}

	ClassifyAndFill(&profile, BuildClassifierInput(&profile))
	return profile
}
