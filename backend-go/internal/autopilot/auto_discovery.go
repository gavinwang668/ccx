package autopilot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/utils"
)

// DiscoveryStatus 发现任务状态枚举。
type DiscoveryStatus string

const (
	DiscoveryStatusIdle    DiscoveryStatus = "idle"
	DiscoveryStatusRunning DiscoveryStatus = "running"
	DiscoveryStatusDone    DiscoveryStatus = "done"
	DiscoveryStatusFailed  DiscoveryStatus = "failed"
)

// EndpointDiscoveryResult 单个 (baseURL, key) 端点的发现结果。
type EndpointDiscoveryResult struct {
	KeyMask      string   `json:"keyMask"`
	BaseURL      string   `json:"baseUrl"`
	ModelsCount  int      `json:"modelsCount"`
	Models       []string `json:"models,omitempty"`
	ProtocolOk   bool     `json:"protocolOk"`
	ErrorMessage string   `json:"errorMessage,omitempty"`
}

// DiscoveryTask 单渠道发现任务的运行时状态。
type DiscoveryTask struct {
	ChannelUID   string                      `json:"channelUid"`
	Status       DiscoveryStatus             `json:"status"`
	StartedAt    *time.Time                  `json:"startedAt,omitempty"`
	FinishedAt   *time.Time                  `json:"finishedAt,omitempty"`
	Error        string                      `json:"error,omitempty"`
	Endpoints    []EndpointDiscoveryResult    `json:"endpoints"`
	cancel       context.CancelFunc          `json:"-"`
}

// AutoDiscoveryRunner 自动发现执行器。
// 内存状态机：每个渠道同时只运行一个发现任务，重复触发会被拒绝。
// 所有配置为空时零值即可用，不触发任何实际操作。
type AutoDiscoveryRunner struct {
	mu      sync.Mutex
	tasks   map[string]*DiscoveryTask // channelUID -> task
	store   *ProfileStore             // nil 时不写画像，只记录结果
	client  *http.Client              // nil 时使用默认 client
	timeout time.Duration             // 单次请求超时，默认 10s
}

// NewAutoDiscoveryRunner 创建发现执行器。
// store 可为 nil（仅记录内存结果，不写持久化画像）。
func NewAutoDiscoveryRunner(store *ProfileStore) *AutoDiscoveryRunner {
	return &AutoDiscoveryRunner{
		tasks:   make(map[string]*DiscoveryTask),
		store:   store,
		timeout: 10 * time.Second,
	}
}

// GetTask 返回指定渠道的发现任务快照（nil 表示从未触发）。
func (r *AutoDiscoveryRunner) GetTask(channelUID string) *DiscoveryTask {
	r.mu.Lock()
	defer r.mu.Unlock()
	task := r.tasks[channelUID]
	if task == nil {
		return nil
	}
	// 返回快照，不暴露 cancel
	snap := *task
	snap.cancel = nil
	// 深拷贝 Endpoints
	if len(task.Endpoints) > 0 {
		snap.Endpoints = make([]EndpointDiscoveryResult, len(task.Endpoints))
		copy(snap.Endpoints, task.Endpoints)
	}
	return &snap
}

// TriggerDiscovery 触发发现任务。
// 如果同渠道已有 running 任务则返回 false（拒绝重复触发）。
// 返回 true 表示已成功触发。
func (r *AutoDiscoveryRunner) TriggerDiscovery(channelUID string, channel *config.UpstreamConfig, cfgManager *config.ConfigManager) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.tasks[channelUID]; ok && existing.Status == DiscoveryStatusRunning {
		log.Printf("[AutoDiscovery-Trigger] 渠道 %s 发现任务已在运行中，拒绝重复触发", channelUID)
		return false
	}

	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now()
	task := &DiscoveryTask{
		ChannelUID: channelUID,
		Status:     DiscoveryStatusRunning,
		StartedAt:  &now,
		cancel:     cancel,
	}
	r.tasks[channelUID] = task

	go r.runDiscovery(ctx, task, channel, cfgManager)
	return true
}

// runDiscovery 执行发现逻辑（在后台 goroutine 中运行）。
func (r *AutoDiscoveryRunner) runDiscovery(ctx context.Context, task *DiscoveryTask, channel *config.UpstreamConfig, _ *config.ConfigManager) {
	defer func() {
		if rec := recover(); rec != nil {
			r.mu.Lock()
			task.Status = DiscoveryStatusFailed
			now := time.Now()
			task.FinishedAt = &now
			task.Error = fmt.Sprintf("panic: %v", rec)
			r.mu.Unlock()
			log.Printf("[AutoDiscovery-Run] 渠道 %s 发现任务 panic: %v", task.ChannelUID, rec)
		}
	}()

	endpoints := r.discoverEndpoints(ctx, channel)

	r.mu.Lock()
	task.Endpoints = endpoints
	now := time.Now()
	task.FinishedAt = &now

	// 检查是否有失败的端点
	failedCount := 0
	for _, ep := range endpoints {
		if !ep.ProtocolOk {
			failedCount++
		}
	}
	if failedCount == len(endpoints) && len(endpoints) > 0 {
		task.Status = DiscoveryStatusFailed
		task.Error = "所有端点均不可达"
	} else {
		task.Status = DiscoveryStatusDone
	}
	r.mu.Unlock()

	// 写画像到 ProfileStore（在锁外执行，避免阻塞其他操作）
	if r.store != nil {
		r.writeProfiles(task.ChannelUID, channel, endpoints)
	}

	log.Printf("[AutoDiscovery-Run] 渠道 %s 发现完成: %d/%d 端点可达",
		task.ChannelUID, len(endpoints)-failedCount, len(endpoints))
}

// discoverEndpoints 遍历所有 (baseURL, key) 组合，调用 GET /v1/models。
func (r *AutoDiscoveryRunner) discoverEndpoints(ctx context.Context, channel *config.UpstreamConfig) []EndpointDiscoveryResult {
	baseURLs := channel.GetAllBaseURLs()
	keys := channel.APIKeys

	if len(baseURLs) == 0 || len(keys) == 0 {
		return nil
	}

	client := r.client
	if client == nil {
		client = &http.Client{Timeout: r.timeout}
	}

	var results []EndpointDiscoveryResult
	for _, baseURL := range baseURLs {
		for _, key := range keys {
			select {
			case <-ctx.Done():
				return results
			default:
			}

			result := r.probeEndpoint(ctx, client, channel, baseURL, key)
			results = append(results, result)
		}
	}
	return results
}

// probeEndpoint 探测单个 (baseURL, key) 组合。
// MVP 实现：调 GET /v1/models 检查协议可达性和模型列表。
func (r *AutoDiscoveryRunner) probeEndpoint(ctx context.Context, client *http.Client, channel *config.UpstreamConfig, baseURL, apiKey string) EndpointDiscoveryResult {
	result := EndpointDiscoveryResult{
		KeyMask: utils.MaskAPIKey(apiKey),
		BaseURL: baseURL,
	}

	// 构建 models URL
	modelsURL := strings.TrimRight(baseURL, "/")
	if channel.ServiceType == "gemini" {
		// Gemini 不支持 /v1/models，跳过
		result.ProtocolOk = false
		result.ErrorMessage = "Gemini 暂不支持 models 探测"
		return result
	}
	modelsURL += "/v1/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("构建请求失败: %v", err)
		return result
	}

	// 设置认证头
	authHeader := channel.AuthHeader
	if authHeader == "" {
		authHeader = "bearer"
	}
	switch strings.ToLower(authHeader) {
	case "x-api-key":
		req.Header.Set("x-api-key", apiKey)
	default:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("请求失败: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		result.ErrorMessage = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		return result
	}

	// 解析 models 响应
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("读取响应失败: %v", err)
		return result
	}

	models := parseModelsResponse(body)
	result.ModelsCount = len(models)
	result.Models = models
	result.ProtocolOk = true

	return result
}

// parseModelsResponse 解析 OpenAI /v1/models 响应体。
func parseModelsResponse(body []byte) []string {
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	models := make([]string, 0, len(resp.Data))
	for _, m := range resp.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	return models
}

// writeProfiles 将发现结果写入 KeyEndpointProfile。
// MVP：只更新 ModelListHash / AvailableModels / Source / UpdatedAt，不修改 modelMapping。
func (r *AutoDiscoveryRunner) writeProfiles(channelUID string, channel *config.UpstreamConfig, endpoints []EndpointDiscoveryResult) {
	if r.store == nil {
		return
	}

	for _, ep := range endpoints {
		if !ep.ProtocolOk {
			continue
		}

		// 从 channel 的 APIKeys 中找到对应 key
		apiKey := ""
		for _, key := range channel.APIKeys {
			if utils.MaskAPIKey(key) == ep.KeyMask {
				apiKey = key
				break
			}
		}
		if apiKey == "" {
			continue
		}

		endpointUID := GenerateEndpointUID(channelUID, ep.BaseURL, KeyHashFromAPIKey(apiKey))

		// 尝试获取已有画像
		existing := r.store.Get(endpointUID)

		var profile KeyEndpointProfile
		if existing != nil {
			profile = *existing
		}

		// 更新发现相关字段
		profile.ChannelUID = channelUID
		profile.BaseURL = ep.BaseURL
		profile.KeyMask = ep.KeyMask
		profile.AvailableModels = ep.Models
		if len(ep.Models) > 0 {
			hash := sha256.Sum256([]byte(strings.Join(ep.Models, ",")))
			profile.ModelListHash = hex.EncodeToString(hash[:8])
		}
		profile.Source = "auto_discovery"
		profile.UpdatedAt = time.Now()

		if err := r.store.Upsert(&profile); err != nil {
			log.Printf("[AutoDiscovery-Profile] 写入画像失败 endpoint=%s: %v", endpointUID, err)
		}
	}
}
