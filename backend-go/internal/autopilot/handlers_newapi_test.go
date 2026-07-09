package autopilot

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/gin-gonic/gin"
)

// ── 测试辅助 ──

func setupNewApiRouter(t *testing.T, deps *NewApiRouteDeps) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterNewApiSubscriptionRoutes(r.Group("/api"), deps)
	return r
}

func setupNewApiTestConfigManager(t *testing.T) *config.ConfigManager {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	cfg := config.Config{}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	cfgManager, err := config.NewConfigManager(configPath, "")
	if err != nil {
		t.Fatalf("创建 ConfigManager 失败: %v", err)
	}
	return cfgManager
}

// mockNewApiSite 启动一个模拟 new-api 站点，支持 verify + list/create token 流程。
// tokens 用闭包状态模拟服务端持久化，便于测试 ProvisionKey 的查重/创建/回退逻辑。
// existingTokenKey 为空字符串时模拟"列表接口对已存在 key 做了脱敏、不回显明文"的上游行为
// （§8.5.1 设计文档中 key 列表 data.items[] 含 key 字段，但部分 fork 可能脱敏）。
func mockNewApiSite(t *testing.T, existingTokenName string, existingTokenKey string, createRespHasKey bool) *httptest.Server {
	t.Helper()
	var created []NewApiToken
	nextID := 100
	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, true, NewApiUserSelf{ID: 7, Username: "bob", Quota: 50000, UsedQuota: 1000}, "")
	})
	mux.HandleFunc("/api/user/self/groups", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, true, map[string]NewApiGroupInfo{"default": {Desc: "默认", Ratio: 1.0}}, "")
	})
	mux.HandleFunc("/api/user/models", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, true, []string{"gpt-4o", "claude-3-5-sonnet"}, "")
	})
	mux.HandleFunc("/api/token/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items := []NewApiToken{}
			if existingTokenName != "" {
				items = append(items, NewApiToken{ID: 1, Key: existingTokenKey, Name: existingTokenName, Status: 1})
			}
			items = append(items, created...)
			writeEnvelope(w, true, newApiTokenListData{Items: items}, "")
		case http.MethodPost:
			var req NewApiCreateTokenRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			nextID++
			tok := NewApiToken{ID: nextID, Name: req.Name, Status: 1}
			if createRespHasKey {
				tok.Key = "sk-newly-created-key"
			}
			created = append(created, tok)
			writeEnvelope(w, true, tok, "")
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ── handleNewApiVerify ──

func TestHandleNewApiVerify_Success(t *testing.T) {
	site := mockNewApiSite(t, "", "", true)
	store, err := NewSubscriptionStoreWithDB(newTestDB(t))
	if err != nil {
		t.Fatalf("创建 store 失败: %v", err)
	}
	router := setupNewApiRouter(t, &NewApiRouteDeps{Store: store})

	body, _ := json.Marshal(NewApiVerifyRequest{
		BaseURL:     site.URL,
		AccessToken: "secret-token-value",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/newapi/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var resp NewApiVerifyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if resp.Username != "bob" || resp.UserID != 7 {
		t.Fatalf("用户信息不匹配: %+v", resp)
	}
	if resp.Groups["default"] != 1.0 {
		t.Fatalf("分组倍率不匹配: %+v", resp.Groups)
	}
	if len(resp.AvailableModels) != 2 {
		t.Fatalf("模型列表不匹配: %+v", resp.AvailableModels)
	}
	// AccessToken 绝不完整出响应
	if resp.AccessTokenMasked == "secret-token-value" {
		t.Fatal("AccessToken 未脱敏就出现在响应中")
	}
	if w.Body.String() == "" || bytesContains(w.Body.Bytes(), []byte("secret-token-value")) {
		t.Fatal("响应体中出现了完整 AccessToken 明文")
	}
}

func TestHandleNewApiVerify_InvalidToken(t *testing.T) {
	site := mockNewApiSite(t, "", "", true)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, false, nil, "invalid token")
	})
	badSite := httptest.NewServer(mux)
	t.Cleanup(badSite.Close)
	_ = site

	store, _ := NewSubscriptionStoreWithDB(newTestDB(t))
	router := setupNewApiRouter(t, &NewApiRouteDeps{Store: store})

	body, _ := json.Marshal(NewApiVerifyRequest{BaseURL: badSite.URL, AccessToken: "bad-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/newapi/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("期望 502, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestHandleNewApiVerify_MissingFields(t *testing.T) {
	store, _ := NewSubscriptionStoreWithDB(newTestDB(t))
	router := setupNewApiRouter(t, &NewApiRouteDeps{Store: store})

	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/newapi/verify", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

// ── handleNewApiProvision ──

func TestHandleNewApiProvision_FullFlow_CreateNewKey(t *testing.T) {
	site := mockNewApiSite(t, "", "", true)
	store, err := NewSubscriptionStoreWithDB(newTestDB(t))
	if err != nil {
		t.Fatalf("创建 store 失败: %v", err)
	}
	cfgManager := setupNewApiTestConfigManager(t)
	runner := NewAutoDiscoveryRunner(nil, nil) // profile store/hub 为 nil，只验证渠道创建路径
	router := setupNewApiRouter(t, &NewApiRouteDeps{Store: store, CfgManager: cfgManager, Runner: runner})

	reqBody := NewApiProvisionRequest{
		SubscriptionUID: "sub-newapi-1",
		DisplayName:     "测试中转站",
		BaseURL:         site.URL,
		AccessToken:     "secret-provision-token",
		ChannelKind:     "messages",
		ChannelName:     "newapi-test-channel",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/newapi/provision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("期望 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var resp NewApiProvisionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if resp.ProvisionedKey != "sk-newly-created-key" {
		t.Fatalf("建 key 结果不匹配: %+v", resp)
	}
	if resp.Reused {
		t.Fatal("期望新建，但标记为 reused")
	}
	if resp.ChannelUID == "" {
		t.Fatal("channelUID 为空")
	}

	// profile 已落库，且 AccessToken 不在响应中完整出现
	profile := store.Get("sub-newapi-1")
	if profile == nil {
		t.Fatal("profile 未创建")
	}
	if profile.AccessToken != "secret-provision-token" {
		t.Fatalf("profile 持久化的 AccessToken 不匹配: got=%s", profile.AccessToken)
	}
	if bytesContains(w.Body.Bytes(), []byte("secret-provision-token")) {
		t.Fatal("响应体中出现了完整 AccessToken 明文")
	}

	// 渠道确实建到了 messages 上游列表
	cfg := cfgManager.GetConfig()
	found := false
	for _, ch := range cfg.Upstream {
		if ch.Name == "newapi-test-channel" {
			found = true
			if len(ch.APIKeys) != 1 || ch.APIKeys[0] != "sk-newly-created-key" {
				t.Fatalf("渠道 APIKeys 不匹配: %+v", ch.APIKeys)
			}
			if ch.ChannelUID != resp.ChannelUID {
				t.Fatalf("渠道 UID 不匹配: cfg=%s resp=%s", ch.ChannelUID, resp.ChannelUID)
			}
		}
	}
	if !found {
		t.Fatal("未在 messages 上游列表中找到新建渠道")
	}
}

func TestHandleNewApiProvision_ReuseExistingKey_Succeeds(t *testing.T) {
	// 站点 key 列表按 §8.5.1 设计返回明文 key（data.items[].key），复用同名 key 时应直接成功建渠道。
	site := mockNewApiSite(t, DefaultNewApiProvisionKeyName, "sk-existing-key", true)
	store, _ := NewSubscriptionStoreWithDB(newTestDB(t))
	cfgManager := setupNewApiTestConfigManager(t)
	runner := NewAutoDiscoveryRunner(nil, nil)
	router := setupNewApiRouter(t, &NewApiRouteDeps{Store: store, CfgManager: cfgManager, Runner: runner})

	reqBody := NewApiProvisionRequest{
		SubscriptionUID: "sub-newapi-2",
		DisplayName:     "测试中转站2",
		BaseURL:         site.URL,
		AccessToken:     "secret-token-2",
		ChannelKind:     "messages",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/newapi/provision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("期望 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var resp NewApiProvisionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if !resp.Reused {
		t.Fatal("期望复用已存在 key，但标记为新建")
	}
	if resp.ProvisionedKey != "sk-existing-key" {
		t.Fatalf("复用 key 不匹配: %+v", resp)
	}
	if store.Get("sub-newapi-2") == nil {
		t.Fatal("复用成功后应创建 profile")
	}
}

func TestHandleNewApiProvision_ReuseExistingKey_MaskedKey_ReturnsConflict(t *testing.T) {
	// 部分 fork 的 key 列表接口不回显明文 key（脱敏/空字符串），此时无法拿到可用 key，应返回 409 让用户手动处理。
	site := mockNewApiSite(t, DefaultNewApiProvisionKeyName, "", true)
	store, _ := NewSubscriptionStoreWithDB(newTestDB(t))
	cfgManager := setupNewApiTestConfigManager(t)
	runner := NewAutoDiscoveryRunner(nil, nil)
	router := setupNewApiRouter(t, &NewApiRouteDeps{Store: store, CfgManager: cfgManager, Runner: runner})

	reqBody := NewApiProvisionRequest{
		SubscriptionUID: "sub-newapi-2",
		DisplayName:     "测试中转站2",
		BaseURL:         site.URL,
		AccessToken:     "secret-token-2",
		ChannelKind:     "messages",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/newapi/provision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("期望 409, got %d, body=%s", w.Code, w.Body.String())
	}
	// profile 不应残留
	if store.Get("sub-newapi-2") != nil {
		t.Fatal("建 key 失败后不应创建 profile")
	}
}

func TestHandleNewApiProvision_DuplicateSubscriptionUID_Rejected(t *testing.T) {
	site := mockNewApiSite(t, "", "", true)
	store, _ := NewSubscriptionStoreWithDB(newTestDB(t))
	cfgManager := setupNewApiTestConfigManager(t)
	runner := NewAutoDiscoveryRunner(nil, nil)
	router := setupNewApiRouter(t, &NewApiRouteDeps{Store: store, CfgManager: cfgManager, Runner: runner})

	existing := &SubscriptionProfile{
		SubscriptionUID: "sub-dup",
		DisplayName:     "已存在",
		Provider:        "manual",
	}
	if err := store.Create(existing); err != nil {
		t.Fatalf("预置 profile 失败: %v", err)
	}

	reqBody := NewApiProvisionRequest{
		SubscriptionUID: "sub-dup",
		DisplayName:     "重复",
		BaseURL:         site.URL,
		AccessToken:     "tok",
		ChannelKind:     "messages",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/newapi/provision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("期望 409, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestHandleNewApiProvision_InvalidChannelKind(t *testing.T) {
	site := mockNewApiSite(t, "", "", true)
	store, _ := NewSubscriptionStoreWithDB(newTestDB(t))
	cfgManager := setupNewApiTestConfigManager(t)
	runner := NewAutoDiscoveryRunner(nil, nil)
	router := setupNewApiRouter(t, &NewApiRouteDeps{Store: store, CfgManager: cfgManager, Runner: runner})

	reqBody := NewApiProvisionRequest{
		SubscriptionUID: "sub-bad-kind",
		DisplayName:     "非法渠道类型",
		BaseURL:         site.URL,
		AccessToken:     "tok",
		ChannelKind:     "not-a-real-kind",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/newapi/provision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestHandleNewApiProvision_MissingCfgManager(t *testing.T) {
	store, _ := NewSubscriptionStoreWithDB(newTestDB(t))
	router := setupNewApiRouter(t, &NewApiRouteDeps{Store: store, CfgManager: nil})

	reqBody := NewApiProvisionRequest{
		SubscriptionUID: "sub-no-cfg",
		DisplayName:     "无配置管理器",
		BaseURL:         "https://example.com",
		AccessToken:     "tok",
		ChannelKind:     "messages",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/newapi/provision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("期望 500, got %d, body=%s", w.Code, w.Body.String())
	}
}

// bytesContains 是 bytes.Contains 的语义化包装，方便断言"响应体不应包含明文令牌"。
func bytesContains(haystack, needle []byte) bool {
	return bytes.Contains(haystack, needle)
}
