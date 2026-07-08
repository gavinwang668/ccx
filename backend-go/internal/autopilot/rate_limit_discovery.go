package autopilot

import (
	"log"
	"math"
	"sync"
	"time"
)

// ── 信号来源类型 ──

// RateLimitSignalSource 表示限速信号的来源类型。
type RateLimitSignalSource string

const (
	SignalSourceHeader  RateLimitSignalSource = "header"  // 从上游响应头解析
	SignalSource429     RateLimitSignalSource = "429"     // 429 响应反推
	SignalSourceSuccess RateLimitSignalSource = "success" // 成功响应（用于 AIMD 上调）
)

// ── 输入信号 ──

// RateLimitSignal 是一次上游响应携带的限速相关信号。
type RateLimitSignal struct {
	// Source 信号来源。
	Source RateLimitSignalSource `json:"source"`

	// ── header 显式值 ──
	// Limit 为 x-ratelimit-limit-requests 或 anthropic-ratelimit-limit-requests 解析值。
	// 表示窗口内请求上限（非 RPM，需结合 Window 换算）。
	Limit int `json:"limit,omitempty"`
	// Remaining 为 x-ratelimit-remaining-requests 或 anthropic-ratelimit-requests-remaining 解析值。
	Remaining int `json:"remaining,omitempty"`
	// ResetSeconds 为 x-ratelimit-reset-requests 解析的重置窗口秒数。
	ResetSeconds float64 `json:"resetSeconds,omitempty"`
	// WindowSeconds 为 header 中指示的限速窗口秒数（如 60s、3600s）。
	// 若 ResetSeconds 已知，可从 ResetSeconds 推算；若 header 明确给出窗口大小则直接填入。
	WindowSeconds float64 `json:"windowSeconds,omitempty"`

	// ── 429 信号 ──
	// HasRetryAfter 是否携带 Retry-After header。
	HasRetryAfter bool `json:"hasRetryAfter,omitempty"`
	// RetryAfterSeconds Retry-After header 解析的秒数。
	RetryAfterSeconds float64 `json:"retryAfterSeconds,omitempty"`

	// ── 成功信号 ──
	// IsStreaming 是否为流式请求（影响并发学习）。
	IsStreaming bool `json:"isStreaming,omitempty"`
	// LatencyMs 响应延迟毫秒（用于判断是否"低延迟稳定"）。
	LatencyMs int64 `json:"latencyMs,omitempty"`

	// Timestamp 信号产生的时间戳。
	Timestamp time.Time `json:"timestamp"`
}

// ── 学习状态 ──

// endpointLearnState 持有单个 endpoint 的限速学习状态。
type endpointLearnState struct {
	// estimatedRPM 当前估算的 RPM。
	EstimatedRPM int `json:"estimatedRpm"`
	// estimatedTPM 当前估算的 TPM（token 每分钟，Phase 1 暂不精细推导）。
	EstimatedTPM int `json:"estimatedTpm,omitempty"`
	// estimatedRPD 当前估算的 RPD。
	EstimatedRPD int `json:"estimatedRpd,omitempty"`
	// windowSeconds 当前学习到的限速窗口秒数。
	WindowSeconds int `json:"windowSeconds"`
	// maxConcurrent 估算的最大并发。
	MaxConcurrent int `json:"maxConcurrent,omitempty"`
	// source 最近一次更新信号的来源。
	Source RateLimitSource `json:"source"`
	// confidence 当前置信度，0.0-1.0。
	Confidence float64 `json:"confidence"`
	// last429At 最近一次 429 信号的时间。
	Last429At *time.Time `json:"last429At,omitempty"`
	// lastSuccessAt 最近一次成功信号的时间。
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`
	// updatedAt 最近一次状态更新时间。
	UpdatedAt time.Time `json:"updatedAt"`
	// observeCount 累计观测次数。
	ObserveCount int `json:"observeCount"`
	// headerSuccessCount header 信号连续成功解析计数（用于提升 confidence）。
	HeaderSuccessCount int `json:"headerSuccessCount"`
	// lastAIMDIncreaseAt 最近一次 AIMD 上调时间。
	LastAIMDIncreaseAt *time.Time `json:"lastAIMDIncreaseAt,omitempty"`
	// no429Since 最近一次 429 后连续无 429 的起始时间。
	No429Since *time.Time `json:"no429Since,omitempty"`
	// consecutiveSuccessesSince429 自最近 429 后连续成功次数。
	ConsecutiveSuccessesSince429 int `json:"consecutiveSuccessesSince429,omitempty"`
}

// ── 配置 ──

// RateLimitDiscovererConfig 可调参数。
type RateLimitDiscovererConfig struct {
	// MinRPM 估算下限，防止降为 0 后永久不可用。默认 1。
	MinRPM int `json:"minRpm"`
	// MaxAutoRPM 无明确 header 时的自动估算上限。默认 120。
	MaxAutoRPM int `json:"maxAutoRpm"`
	// ConfidenceThreshold 建议被采纳的最低置信度阈值。默认 0.3。
	ConfidenceThreshold float64 `json:"confidenceThreshold"`
	// AIMDIncreaseInterval AIMD 上调最短间隔。默认 10 分钟。
	AIMDIncreaseInterval time.Duration `json:"aimdIncreaseInterval"`
	// AIMDIncreasePercent AIMD 上调幅度百分比。默认 10。
	AIMDIncreasePercent float64 `json:"aimdIncreasePercent"`
	// AIMDNo429Grace AIMD 上调要求最近无 429 的窗口。默认 15 分钟。
	AIMDNo429Grace time.Duration `json:"aimdNo429Grace"`
	// HeaderConfidenceMax header 来源置信度上限。默认 0.9。
	HeaderConfidenceMax float64 `json:"headerConfidenceMax"`
	// RemainingConfidenceMax remaining/reset 推导置信度上限。默认 0.75。
	RemainingConfidenceMax float64 `json:"remainingConfidenceMax"`
	// QuietLogs 是否静默日志。
	QuietLogs bool `json:"quietLogs"`
}

// defaultDiscovererConfig 返回默认配置。
func defaultDiscovererConfig() RateLimitDiscovererConfig {
	return RateLimitDiscovererConfig{
		MinRPM:                 1,
		MaxAutoRPM:             120,
		ConfidenceThreshold:    0.3,
		AIMDIncreaseInterval:   10 * time.Minute,
		AIMDIncreasePercent:    10,
		AIMDNo429Grace:         15 * time.Minute,
		HeaderConfidenceMax:    0.9,
		RemainingConfidenceMax: 0.75,
		QuietLogs:              false,
	}
}

// ── Discoverer 主体 ──

// RateLimitDiscoverer 在 shadow/read-only 模式下推导 endpoint 的限速建议。
// Phase 1: 只输出建议（SuggestedLimit），不写任何 limiter。
// 并发安全，状态可 JSON 序列化。
type RateLimitDiscoverer struct {
	mu      sync.RWMutex
	states  map[string]*endpointLearnState // key: endpointUID
	cfg     RateLimitDiscovererConfig
	nowFunc func() time.Time // 可注入，测试用
}

// NewRateLimitDiscoverer 创建 RateLimitDiscoverer。
func NewRateLimitDiscoverer(cfg RateLimitDiscovererConfig) *RateLimitDiscoverer {
	if cfg.MinRPM <= 0 {
		cfg.MinRPM = defaultDiscovererConfig().MinRPM
	}
	if cfg.MaxAutoRPM <= 0 {
		cfg.MaxAutoRPM = defaultDiscovererConfig().MaxAutoRPM
	}
	if cfg.ConfidenceThreshold <= 0 {
		cfg.ConfidenceThreshold = defaultDiscovererConfig().ConfidenceThreshold
	}
	if cfg.AIMDIncreaseInterval <= 0 {
		cfg.AIMDIncreaseInterval = defaultDiscovererConfig().AIMDIncreaseInterval
	}
	if cfg.AIMDIncreasePercent <= 0 {
		cfg.AIMDIncreasePercent = defaultDiscovererConfig().AIMDIncreasePercent
	}
	if cfg.AIMDNo429Grace <= 0 {
		cfg.AIMDNo429Grace = defaultDiscovererConfig().AIMDNo429Grace
	}
	if cfg.HeaderConfidenceMax <= 0 {
		cfg.HeaderConfidenceMax = defaultDiscovererConfig().HeaderConfidenceMax
	}
	if cfg.RemainingConfidenceMax <= 0 {
		cfg.RemainingConfidenceMax = defaultDiscovererConfig().RemainingConfidenceMax
	}
	return &RateLimitDiscoverer{
		states:  make(map[string]*endpointLearnState),
		cfg:     cfg,
		nowFunc: time.Now,
	}
}

// ── SuggestedLimit 输出结构 ──

// SuggestedLimitResult 限速建议结果。
type SuggestedLimitResult struct {
	RPM        int             `json:"rpm"`
	TPM        int             `json:"tpm,omitempty"`
	RPD        int             `json:"rpd,omitempty"`
	Confidence float64         `json:"confidence"`
	Source     RateLimitSource `json:"source"`
}

// ── 公开方法 ──

// Observe 累积一个限速信号。并发安全。
// 推导规则按设计 §4.5.3：header 显式值优先 → 429 反推 → 被动 AIMD 收敛。
func (d *RateLimitDiscoverer) Observe(endpointUID string, sig RateLimitSignal) {
	if endpointUID == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	state, ok := d.states[endpointUID]
	if !ok {
		state = &endpointLearnState{
			WindowSeconds: 60, // 默认 1 分钟窗口
			UpdatedAt:     d.nowFunc(),
		}
		d.states[endpointUID] = state
	}

	now := d.nowFunc()
	state.ObserveCount++
	state.UpdatedAt = now

	switch sig.Source {
	case SignalSourceHeader:
		d.observeHeader(state, sig, now)
	case SignalSource429:
		d.observe429(state, sig, now)
	case SignalSourceSuccess:
		d.observeSuccess(state, sig, now)
	}
}

// SuggestedLimit 返回 endpoint 的限速建议。并发安全。
// 返回 zero value（rpm=0, confidence=0, source=unknown）表示信号不足，无建议。
func (d *RateLimitDiscoverer) SuggestedLimit(endpointUID string) SuggestedLimitResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	state, ok := d.states[endpointUID]
	if !ok || state.ObserveCount == 0 {
		return SuggestedLimitResult{
			Source: RateLimitSourceUnknown,
		}
	}

	return SuggestedLimitResult{
		RPM:        state.EstimatedRPM,
		TPM:        state.EstimatedTPM,
		RPD:        state.EstimatedRPD,
		Confidence: state.Confidence,
		Source:     state.Source,
	}
}

// AllSuggestedLimits 返回所有已观测 endpoint 的限速建议。并发安全。
func (d *RateLimitDiscoverer) AllSuggestedLimits() map[string]SuggestedLimitResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]SuggestedLimitResult, len(d.states))
	for uid, state := range d.states {
		if state.ObserveCount == 0 {
			continue
		}
		result[uid] = SuggestedLimitResult{
			RPM:        state.EstimatedRPM,
			TPM:        state.EstimatedTPM,
			RPD:        state.EstimatedRPD,
			Confidence: state.Confidence,
			Source:     state.Source,
		}
	}
	return result
}

// GetState 返回指定 endpoint 的学习状态快照（深拷贝），供序列化/调试。
// 返回 nil 表示该 endpoint 无观测记录。
func (d *RateLimitDiscoverer) GetState(endpointUID string) *endpointLearnState {
	d.mu.RLock()
	defer d.mu.RUnlock()

	state, ok := d.states[endpointUID]
	if !ok {
		return nil
	}
	cp := *state
	return &cp
}

// StateCount 返回当前跟踪的 endpoint 数量。
func (d *RateLimitDiscoverer) StateCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.states)
}

// ── 内部推导逻辑（§4.5.3）──

// observeHeader 处理 header 显式 limit 信号。
// 规则：estimatedRPM = normalize(limit, reset/window)，confidence = 0.9。
func (d *RateLimitDiscoverer) observeHeader(state *endpointLearnState, sig RateLimitSignal, now time.Time) {
	if sig.Limit > 0 {
		// header 明确给 limit：换算为 RPM
		window := sig.WindowSeconds
		if window <= 0 && sig.ResetSeconds > 0 {
			window = sig.ResetSeconds
		}
		if window <= 0 {
			window = 60 // 默认 1 分钟窗口
		}
		state.WindowSeconds = int(window)
		rpm := normalizeToRPM(sig.Limit, window)
		rpm = d.clampRPM(rpm)
		state.EstimatedRPM = rpm
		state.Source = RateLimitSourceHeader
		state.Confidence = d.cfg.HeaderConfidenceMax

		if !d.cfg.QuietLogs {
			log.Printf("[RateLimitDiscover-Header] endpoint=%s limit=%d window=%.0fs -> rpm=%d confidence=%.2f",
				"", sig.Limit, window, rpm, state.Confidence)
		}
		return
	}

	// 只有 remaining/reset：估算当前消耗速度
	if sig.Remaining >= 0 && sig.ResetSeconds > 0 {
		state.WindowSeconds = int(sig.ResetSeconds)
		// observedRate = 已消耗量 / 已过去时间，但这里只有 remaining 和 reset，
		// 用 capacity = remaining + consumed 近似。
		// 更保守地：如果 remaining 很低，说明 capacity 接近 remaining + 已知消耗。
		// 这里简化为：inferred_capacity = remaining + (剩余窗口内的预估消耗)
		// 保守策略：以 reset 窗口的 remaining 推算 RPM 上限
		if sig.ResetSeconds > 0 {
			// remaining 是 reset 窗口内剩余配额，reset 是距重置的时间
			// 推算：窗口总容量 >= remaining，假设已消耗的量 = elapsed 内的消耗
			// 这里无法精确得知窗口总容量，保守估计 remaining 就是上限残余
			// 最安全的推算：RPM <= remaining / (resetSeconds / 60)
			resetMinutes := sig.ResetSeconds / 60.0
			if resetMinutes > 0 {
				inferredRPM := int(float64(sig.Remaining) / resetMinutes)
				if inferredRPM > 0 {
					// 取 min(当前估算, 推算值) —— 只降不升
					if state.EstimatedRPM == 0 || inferredRPM < state.EstimatedRPM {
						state.EstimatedRPM = d.clampRPM(inferredRPM)
					}
					state.Source = RateLimitSourceHeader
				}
			}
		}

		// 逐次成功解析后提升 confidence，最高 RemainingConfidenceMax
		state.HeaderSuccessCount++
		newConf := math.Min(
			float64(state.HeaderSuccessCount)*0.15,
			d.cfg.RemainingConfidenceMax,
		)
		if newConf > state.Confidence {
			state.Confidence = newConf
		}
	}
}

// observe429 处理 429 信号。
// 规则：有 Retry-After → cooldown 语义 + 估算 = floor(current * 0.5)；无 Retry-After → floor(current * 0.7)。
func (d *RateLimitDiscoverer) observe429(state *endpointLearnState, sig RateLimitSignal, now time.Time) {
	state.Last429At = &now
	// 重置连续成功计数
	state.ConsecutiveSuccessesSince429 = 0
	state.No429Since = nil

	currentRPM := state.EstimatedRPM
	if currentRPM <= 0 {
		// 还没有基线估算，用 MaxAutoRPM 的一半作为起点
		currentRPM = d.cfg.MaxAutoRPM / 2
	}

	if sig.HasRetryAfter && sig.RetryAfterSeconds > 0 {
		// 429 + Retry-After: 估算 = floor(current * 0.5)，confidence >= 0.7
		newRPM := int(math.Floor(float64(currentRPM) * 0.5))
		if newRPM < d.cfg.MinRPM {
			newRPM = d.cfg.MinRPM
		}
		state.EstimatedRPM = newRPM
		state.Source = RateLimitSourcePassiveAIMD
		if state.Confidence < 0.7 {
			state.Confidence = 0.7
		}

		if !d.cfg.QuietLogs {
			log.Printf("[RateLimitDiscover-429] endpoint=unknown 429+Retry-After=%.0fs rpm: %d -> %d confidence=%.2f",
				sig.RetryAfterSeconds, currentRPM, newRPM, state.Confidence)
		}
	} else {
		// 429 无 Retry-After: 估算 = floor(current * 0.7)，confidence >= 0.5
		newRPM := int(math.Floor(float64(currentRPM) * 0.7))
		if newRPM < d.cfg.MinRPM {
			newRPM = d.cfg.MinRPM
		}
		state.EstimatedRPM = newRPM
		state.Source = RateLimitSourcePassiveAIMD
		if state.Confidence < 0.5 {
			state.Confidence = 0.5
		}

		if !d.cfg.QuietLogs {
			log.Printf("[RateLimitDiscover-429] endpoint=unknown 429 rpm: %d -> %d confidence=%.2f",
				currentRPM, newRPM, state.Confidence)
		}
	}
}

// observeSuccess 处理成功信号，用于 AIMD 缓慢上调。
// 规则：每 10 分钟最多 +10%，且需要最近 15 分钟无 429。
func (d *RateLimitDiscoverer) observeSuccess(state *endpointLearnState, sig RateLimitSignal, now time.Time) {
	if sig.LatencyMs > 0 {
		state.LastSuccessAt = &now
	}

	// 更新 No429Since
	if state.Last429At != nil {
		state.ConsecutiveSuccessesSince429++
		if state.No429Since == nil {
			// 从最近 429 时间开始算起
			since := *state.Last429At
			state.No429Since = &since
		}
	} else {
		// 从未见过 429
		if state.No429Since == nil {
			state.No429Since = &now
		}
	}

	// AIMD 上调条件：
	// 1. 有基线估算
	// 2. 距上次上调 >= AIMDIncreaseInterval
	// 3. 最近 AIMDNo429Grace 内无 429
	// 4. 估算未超过 MaxAutoRPM
	if state.EstimatedRPM <= 0 {
		return
	}
	if state.EstimatedRPM >= d.cfg.MaxAutoRPM {
		return
	}

	// 检查上调间隔
	if state.LastAIMDIncreaseAt != nil {
		if now.Sub(*state.LastAIMDIncreaseAt) < d.cfg.AIMDIncreaseInterval {
			return
		}
	}

	// 检查无 429 窗口
	if state.No429Since != nil {
		if now.Sub(*state.No429Since) < d.cfg.AIMDNo429Grace {
			return
		}
	}

	// 满足上调条件：+10%
	increase := math.Max(1, float64(state.EstimatedRPM)*d.cfg.AIMDIncreasePercent/100.0)
	newRPM := state.EstimatedRPM + int(increase)
	newRPM = d.clampRPM(newRPM)

	if newRPM > state.EstimatedRPM {
		state.EstimatedRPM = newRPM
		state.Source = RateLimitSourcePassiveAIMD
		state.LastAIMDIncreaseAt = &now

		if !d.cfg.QuietLogs {
			log.Printf("[RateLimitDiscover-AIMD] endpoint=unknown AIMD increase rpm: %d -> %d",
				state.EstimatedRPM-int(increase), newRPM)
		}
	}
}

// ── 辅助函数 ──

// normalizeToRPM 将窗口内 limit 值换算为 RPM。
func normalizeToRPM(limit int, windowSeconds float64) int {
	if windowSeconds <= 0 {
		return limit
	}
	// 换算：limit / (window / 60)
	minutes := windowSeconds / 60.0
	return int(math.Round(float64(limit) / minutes))
}

// clampRPM 将 RPM 限制在 [MinRPM, MaxAutoRPM] 范围内。
func (d *RateLimitDiscoverer) clampRPM(rpm int) int {
	if rpm < d.cfg.MinRPM {
		return d.cfg.MinRPM
	}
	if rpm > d.cfg.MaxAutoRPM {
		return d.cfg.MaxAutoRPM
	}
	return rpm
}
