package autopilot

import (
	"log"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/ratelimit"
)

// RateLimitApplier 将 RateLimitDiscoverer 的高置信建议应用到运行态 limiter。
// 门控由 config.RateLimitDiscovery.Enabled 和 kill switch 控制：
//   - 开关关闭 / kill switch 生效时：清除所有已注入的发现 RPM
//   - 开关打开时：周期性从 discoverer 取高置信建议，调用 ApplyDiscoveredLimit
//
// 设计 §4.5.4：显式配置永远优先，只有「用户未显式设置 RPM」的 endpoint 才会被注入。
// 并发安全：由调用方控制调用时机（集成阶段由 worker 节奏驱动）。
type RateLimitApplier struct {
	discoverer   *RateLimitDiscoverer
	limiterMgr   *ratelimit.Manager
	configGetter func() config.AutopilotRoutingConfig

	// endpointToLimiterKeys 维护 endpointUID → limiter key 的映射。
	// 由集成层在 GatherAndApply 时传入。
	// limiter key 格式与 ratelimit.Manager 一致（apiType:channelIndex 或 apiType:channelIndex:scope）。
	endpointToLimiterKeys map[string]string

	// lastApplied 快照上次应用的发现 RPM，用于减少重复 UpdateConfig 开销。
	// key = endpointUID, value = 已应用的 RPM。
	lastApplied map[string]int

	quietLogs bool
}

// NewRateLimitApplier 创建 RateLimitApplier。
// discoverer 为 nil 时 Apply 为 no-op。
// limiterMgr 为 nil 时 Apply 为 no-op。
// configGetter 为 nil 时 Apply 为 no-op。
func NewRateLimitApplier(
	discoverer *RateLimitDiscoverer,
	limiterMgr *ratelimit.Manager,
	configGetter func() config.AutopilotRoutingConfig,
	quietLogs bool,
) *RateLimitApplier {
	return &RateLimitApplier{
		discoverer:            discoverer,
		limiterMgr:            limiterMgr,
		configGetter:          configGetter,
		endpointToLimiterKeys: make(map[string]string),
		lastApplied:           make(map[string]int),
		quietLogs:             quietLogs,
	}
}

// EndpointLimiterMapping 表示一个 endpoint 到 limiter key 的映射关系。
type EndpointLimiterMapping struct {
	// EndpointUID autopilot 的 endpoint 唯一标识。
	EndpointUID string
	// LimiterKey ratelimit.Manager 中的 limiter key（apiType:channelIndex 或带 scope）。
	LimiterKey string
	// ExplicitRPM 标记该 endpoint 的 RPM 是否来自用户显式配置。
	// true 时跳过发现 RPM 注入。
	ExplicitRPM bool
}

// SetEndpointMappings 批量设置 endpoint → limiter key 映射。
// 每次 worker 轮询时由集成层调用，替换整个映射表。
func (a *RateLimitApplier) SetEndpointMappings(mappings []EndpointLimiterMapping) {
	if a == nil {
		return
	}
	a.endpointToLimiterKeys = make(map[string]string, len(mappings))
	for _, m := range mappings {
		if m.EndpointUID != "" && m.LimiterKey != "" {
			a.endpointToLimiterKeys[m.EndpointUID] = m.LimiterKey
		}
	}
}

// Apply 执行一次发现 RPM 的应用周期。
//
// 行为（按优先级）：
//  1. discoverer / limiterMgr / configGetter 任一为 nil → no-op
//  2. KillSwitch=true 或 Enabled=false → ClearAllDiscoveredLimits
//  3. Enabled=true → 遍历高置信建议，对未显式配置 RPM 的 endpoint 注入发现 RPM
//
// 并发安全：非线程安全，由集成层保证单线程调用（worker 节奏）。
func (a *RateLimitApplier) Apply() {
	if a == nil || a.discoverer == nil || a.limiterMgr == nil || a.configGetter == nil {
		return
	}

	cfg := a.configGetter()
	killSwitch := cfg.KillSwitch
	rlCfg := cfg.RateLimitDiscovery
	enabled := rlCfg.Enabled && !killSwitch

	if !enabled {
		a.clearAllDiscoveredLimits()
		return
	}

	confidenceThreshold := rlCfg.ConfidenceThreshold
	if confidenceThreshold <= 0 {
		confidenceThreshold = 0.7 // 安全回退
	}

	suggestions := a.discoverer.AllSuggestedLimits()
	applied := 0

	for endpointUID, suggestion := range suggestions {
		// 低置信度跳过
		if suggestion.Confidence < confidenceThreshold {
			continue
		}
		if suggestion.RPM <= 0 {
			continue
		}

		limiterKey, ok := a.endpointToLimiterKeys[endpointUID]
		if !ok {
			continue
		}

		// 检查是否需要更新（避免重复 apply）
		if prev, exists := a.lastApplied[endpointUID]; exists && prev == suggestion.RPM {
			continue
		}

		// ApplyDiscoveredLimit 会检查 explicitRPM，如果显式配置则忽略
		a.applyDiscoveredLimit(limiterKey, suggestion.RPM)
		a.lastApplied[endpointUID] = suggestion.RPM
		applied++
	}

	if applied > 0 && !a.quietLogs {
		log.Printf("[RateLimitApplier-Apply] 应用发现 RPM: %d 个 endpoint", applied)
	}
}

// applyDiscoveredLimit 对单个 limiter 应用发现 RPM。
// limiterKey 为 ratelimit.Manager 的 key 格式。
// ratelimit.Manager 会自动 GetOrCreate，但我们这里只用已有的 limiter（避免创建空壳）。
func (a *RateLimitApplier) applyDiscoveredLimit(limiterKey string, rpm int) {
	// 解析 limiterKey 得到 apiType 和 channelIndex
	apiType, channelIndex, scope := parseLimiterKey(limiterKey)
	if apiType == "" {
		return
	}

	var l *ratelimit.ChannelLimiter
	if scope != "" {
		l = a.limiterMgr.GetScoped(apiType, channelIndex, scope)
	} else {
		l = a.limiterMgr.Get(apiType, channelIndex)
	}
	if l == nil {
		return
	}

	l.SetDiscoveredRPM(rpm)
}

// clearAllDiscoveredLimits 清除所有已注入的发现 RPM。
// Kill switch 或开关关闭时调用，确保不遗留运行态覆盖。
func (a *RateLimitApplier) clearAllDiscoveredLimits() {
	if len(a.lastApplied) == 0 {
		return
	}

	cleared := 0
	for endpointUID, limiterKey := range a.endpointToLimiterKeys {
		if _, exists := a.lastApplied[endpointUID]; !exists {
			continue
		}

		apiType, channelIndex, scope := parseLimiterKey(limiterKey)
		if apiType == "" {
			continue
		}

		var l *ratelimit.ChannelLimiter
		if scope != "" {
			l = a.limiterMgr.GetScoped(apiType, channelIndex, scope)
		} else {
			l = a.limiterMgr.Get(apiType, channelIndex)
		}
		if l != nil {
			l.ClearDiscoveredRPM()
		}
		cleared++
	}

	// 清空 lastApplied 快照
	a.lastApplied = make(map[string]int)

	if cleared > 0 && !a.quietLogs {
		log.Printf("[RateLimitApplier-Clear] 已清除 %d 个发现 RPM（开关关闭或 kill switch）", cleared)
	}
}

// parseLimiterKey 解析 ratelimit.Manager 的 key 格式。
// 支持 "apiType:channelIndex" 和 "apiType:channelIndex:scope" 两种格式。
func parseLimiterKey(key string) (apiType string, channelIndex int, scope string) {
	colon1 := -1
	colon2 := -1
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			if colon1 == -1 {
				colon1 = i
			} else {
				colon2 = i
				break
			}
		}
	}
	if colon1 == -1 {
		return key, 0, ""
	}

	apiType = key[:colon1]
	idx := 0
	for i := colon1 + 1; i < len(key) && key[i] >= '0' && key[i] <= '9'; i++ {
		idx = idx*10 + int(key[i]-'0')
	}

	if colon2 > colon1 {
		scope = key[colon2+1:]
	}

	return apiType, idx, scope
}
