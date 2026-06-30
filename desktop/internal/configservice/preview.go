package configservice

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (s *Service) previewApplyClaude(req ApplyAgentConfigRequest, port int, accessKey string) (ConfigDiffResult, error) {
	provider := normalizeClaudeProvider(req.Provider)
	baseURL, authToken, apiKey, err := resolveClaudeProvider(req, port, accessKey)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	path := s.claudeSettingsPath()
	data, _, err := readJSONMap(path)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	oldData := copyJSONMap(data)

	env, _ := data["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
		data["env"] = env
	}
	originalModel, modelOK := env["ANTHROPIC_MODEL"].(string)
	originalSmallFast, smallFastOK := env["ANTHROPIC_SMALL_FAST_MODEL"].(string)
	env["ANTHROPIC_BASE_URL"] = baseURL
	if authToken != "" {
		env["ANTHROPIC_AUTH_TOKEN"] = authToken
	} else {
		delete(env, "ANTHROPIC_AUTH_TOKEN")
	}
	if apiKey != "" {
		env["ANTHROPIC_API_KEY"] = apiKey
	} else {
		delete(env, "ANTHROPIC_API_KEY")
	}
	applyClaudeProviderModelEnv(env, provider, optionalString(originalModel, modelOK), optionalString(originalSmallFast, smallFastOK))
	newData := data

	files := []FileDiff{computeJSONDiffWithMask(path, oldData, newData, sensitiveFieldKeys...)}

	if provider == ProviderCCX {
		rootPath := s.claudeRootConfigPath()
		oldRoot, _, _ := readJSONMap(rootPath)
		newRoot := copyJSONMap(oldRoot)
		newRoot["hasCompletedOnboarding"] = true
		files = append(files, computeJSONDiff(rootPath, oldRoot, newRoot))
	}
	return ConfigDiffResult{Files: files}, nil
}

func (s *Service) previewApplyCodex(port int, accessKey string, mode string) (ConfigDiffResult, error) {
	if mode == "plugin" {
		return s.previewApplyCodexPlugin(port, accessKey)
	}
	return s.previewApplyCodexQuick(port, accessKey)
}

func (s *Service) previewApplyCodexQuick(port int, accessKey string) (ConfigDiffResult, error) {
	configPath := s.codexConfigPath()
	authPath := s.codexAuthPath()

	configContent, _, err := readTextFile(configPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	authData, _, err := readJSONMap(authPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}

	targetURL := codexBaseURL(port)
	updatedConfig := upsertTopLevelTomlString(configContent, "model_provider", "openai")
	updatedConfig = upsertTopLevelTomlString(updatedConfig, "openai_base_url", targetURL)
	// 清理插件模式残留
	updatedConfig = restoreNamedTomlBlock(updatedConfig, "model_providers.ccx", nil)

	newAuthData := copyJSONMap(authData)
	newAuthData["OPENAI_API_KEY"] = accessKey
	newAuthData["auth_mode"] = "apikey"

	return ConfigDiffResult{Files: []FileDiff{
		computeTextDiffWithSensitiveFields(configPath, configContent, updatedConfig, "experimental_bearer_token"),
		computeJSONDiffWithMask(authPath, authData, newAuthData, "OPENAI_API_KEY"),
	}}, nil
}

func (s *Service) previewApplyCodexPlugin(port int, accessKey string) (ConfigDiffResult, error) {
	configPath := s.codexConfigPath()
	authPath := s.codexAuthPath()
	configContent, _, err := readTextFile(configPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	authData, _, err := readJSONMap(authPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}

	block := fmt.Sprintf(`[model_providers.ccx]
name = "CCX Proxy"
base_url = %q
wire_api = "responses"
requires_openai_auth = true
experimental_bearer_token = %q
`, codexBaseURL(port), accessKey)

	updatedConfig := upsertTopLevelTomlString(configContent, "model_provider", "ccx")
	updatedConfig = restoreTopLevelTomlString(updatedConfig, "openai_base_url", nil)
	updatedConfig = restoreNamedTomlBlock(updatedConfig, "model_providers.ccx", nil)
	updatedConfig = upsertNamedTomlBlock(updatedConfig, "model_providers.ccx", block)

	newAuthData := copyJSONMap(authData)
	newAuthData["OPENAI_API_KEY"] = accessKey
	newAuthData["auth_mode"] = "chatgpt"

	return ConfigDiffResult{Files: []FileDiff{
		computeTextDiffWithSensitiveFields(configPath, configContent, updatedConfig, "experimental_bearer_token"),
		computeJSONDiffWithMask(authPath, authData, newAuthData, "OPENAI_API_KEY"),
	}}, nil
}

func (s *Service) previewApplyCodexOpenAI(apiKey string) (ConfigDiffResult, error) {
	configPath := s.codexConfigPath()
	authPath := s.codexAuthPath()
	configContent, _, err := readTextFile(configPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	authData, _, err := readJSONMap(authPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}

	key := strings.TrimSpace(apiKey)
	newAuthData := copyJSONMap(authData)
	if key != "" {
		// API Key 模式：写入 key + auth_mode = "apikey"
		newAuthData["OPENAI_API_KEY"] = key
		newAuthData["auth_mode"] = "apikey"
	} else {
		// OAuth 登录模式：auth_mode = "chatgpt"，OPENAI_API_KEY = null
		newAuthData["auth_mode"] = "chatgpt"
		newAuthData["OPENAI_API_KEY"] = nil
	}

	updatedConfig := upsertTopLevelTomlString(configContent, "model_provider", "openai")
	updatedConfig = restoreTopLevelTomlString(updatedConfig, "openai_base_url", nil)
	updatedConfig = restoreNamedTomlBlock(updatedConfig, "model_providers.ccx", nil)
	updatedConfig = restoreNamedTomlBlock(updatedConfig, "model_providers.openai", nil)

	return ConfigDiffResult{Files: []FileDiff{
		computeTextDiffWithSensitiveFields(configPath, configContent, updatedConfig, "experimental_bearer_token"),
		computeJSONDiffWithMask(authPath, authData, newAuthData, "OPENAI_API_KEY"),
	}}, nil
}

func (s *Service) previewApplyCodexThirdParty(provider, baseURL, apiKey string) (ConfigDiffResult, error) {
	configPath := s.codexConfigPath()
	authPath := s.codexAuthPath()
	configContent, _, err := readTextFile(configPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	authData, _, err := readJSONMap(authPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}

	key := strings.TrimSpace(apiKey)
	if key == "" {
		key = s.GetSavedProviderKeys()["codex:"+provider]
	}
	if key == "" {
		key = "[未配置]"
	}

	block := fmt.Sprintf(`[model_providers.%s]
name = %q
base_url = %q
wire_api = "responses"
requires_openai_auth = true
experimental_bearer_token = %q
`, provider, provider, baseURL, key)

	updatedConfig := upsertTopLevelTomlString(configContent, "model_provider", provider)
	updatedConfig = restoreTopLevelTomlString(updatedConfig, "openai_base_url", nil) // 清理 CCX proxy 残留
	updatedConfig = restoreNamedTomlBlock(updatedConfig, "model_providers.ccx", nil)
	updatedConfig = restoreNamedTomlBlock(updatedConfig, "model_providers.openai", nil)
	updatedConfig = upsertNamedTomlBlock(updatedConfig, "model_providers."+provider, block)

	newAuthData := copyJSONMap(authData)
	newAuthData["OPENAI_API_KEY"] = key
	newAuthData["auth_mode"] = "chatgpt"

	return ConfigDiffResult{Files: []FileDiff{
		computeTextDiffWithSensitiveFields(configPath, configContent, updatedConfig, "experimental_bearer_token"),
		computeJSONDiffWithMask(authPath, authData, newAuthData, "OPENAI_API_KEY"),
	}}, nil
}

func (s *Service) previewApplyCodexThirdPartyQuick(provider, baseURL, apiKey string) (ConfigDiffResult, error) {
	configPath := s.codexConfigPath()
	authPath := s.codexAuthPath()
	configContent, _, err := readTextFile(configPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	authData, _, err := readJSONMap(authPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}

	key := strings.TrimSpace(apiKey)
	if key == "" {
		key = s.GetSavedProviderKeys()["codex:"+provider]
	}
	if key == "" {
		key = "[未配置]"
	}

	// config.toml: model_provider="openai" + openai_base_url=<第三方 URL>
	updatedConfig := upsertTopLevelTomlString(configContent, "model_provider", "openai")
	updatedConfig = upsertTopLevelTomlString(updatedConfig, "openai_base_url", baseURL)
	updatedConfig = restoreNamedTomlBlock(updatedConfig, "model_providers.ccx", nil)
	// 清理插件模式残留的第三方 provider 块
	if isCodexThirdPartyProvider(provider) {
		updatedConfig = restoreNamedTomlBlock(updatedConfig, "model_providers."+provider, nil)
	}

	newAuthData := copyJSONMap(authData)
	newAuthData["OPENAI_API_KEY"] = key
	newAuthData["auth_mode"] = "apikey"

	return ConfigDiffResult{Files: []FileDiff{
		computeTextDiffWithSensitiveFields(configPath, configContent, updatedConfig, "experimental_bearer_token"),
		computeJSONDiffWithMask(authPath, authData, newAuthData, "OPENAI_API_KEY"),
	}}, nil
}

func (s *Service) previewRestoreClaude() (ConfigDiffResult, error) {
	var state ClaudeProxyState
	if err := readJSONFile(s.claudeStatePath(), &state); err != nil {
		return ConfigDiffResult{}, fmt.Errorf("未找到 Claude 配置状态，请先应用配置")
	}
	path := state.TargetPath
	if !state.FileExisted {
		data, _, _ := readJSONMap(path)
		return ConfigDiffResult{Files: []FileDiff{
			computeJSONDiffWithMask(path, data, nil, sensitiveFieldKeys...),
		}}, nil
	}
	data, _, err := readJSONMap(path)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	oldData := copyJSONMap(data)

	env, _ := data["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
		data["env"] = env
	}
	restoreStringField(env, "ANTHROPIC_BASE_URL", state.OriginalBaseURL)
	restoreStringField(env, "ANTHROPIC_AUTH_TOKEN", state.OriginalAuthToken)
	restoreStringField(env, "ANTHROPIC_API_KEY", state.OriginalAPIKey)
	restoreStringField(env, "ANTHROPIC_MODEL", state.OriginalModel)
	restoreStringField(env, "ANTHROPIC_SMALL_FAST_MODEL", state.OriginalSmallFast)
	if !state.EnvExisted && len(env) == 0 {
		delete(data, "env")
	}

	return ConfigDiffResult{Files: []FileDiff{
		computeJSONDiffWithMask(path, oldData, data, sensitiveFieldKeys...),
	}}, nil
}

func (s *Service) previewRestoreCodex() (ConfigDiffResult, error) {
	var state CodexProxyState
	if err := readJSONFile(s.codexStatePath(), &state); err != nil {
		return ConfigDiffResult{}, fmt.Errorf("未找到 Codex 配置状态，请先应用配置")
	}

	var files []FileDiff

	// config.toml
	if state.ConfigFileExisted {
		content, _, err := readTextFile(state.ConfigPath)
		if err != nil {
			return ConfigDiffResult{}, err
		}
		restoredContent := restoreTopLevelTomlString(content, "model_provider", state.OriginalModelProvider)
		restoredContent = restoreTopLevelTomlString(restoredContent, "openai_base_url", state.OriginalOpenAIBaseURL)
		restoredContent = restoreNamedTomlBlock(restoredContent, "model_providers.ccx", state.OriginalProviderBlock)
		if state.InjectedProvider != "" && state.InjectedProvider != ProviderCCX && state.InjectedProvider != ProviderOpenAI {
			restoredContent = restoreNamedTomlBlock(restoredContent, "model_providers."+state.InjectedProvider, nil)
		}
		files = append(files, computeTextDiff(state.ConfigPath, content, restoredContent))
	} else {
		content, _, _ := readTextFile(state.ConfigPath)
		files = append(files, computeTextDiff(state.ConfigPath, content, ""))
	}

	// auth.json
	if state.AuthFileExisted {
		authData, _, err := readJSONMap(state.AuthPath)
		if err != nil {
			return ConfigDiffResult{}, err
		}
		restoredAuth := copyJSONMap(authData)
		restoreStringField(restoredAuth, "OPENAI_API_KEY", state.OriginalOpenAIAPIKey)
		files = append(files, computeJSONDiffWithMask(state.AuthPath, authData, restoredAuth, "OPENAI_API_KEY"))
	} else {
		authData, _, _ := readJSONMap(state.AuthPath)
		files = append(files, computeJSONDiffWithMask(state.AuthPath, authData, nil, "OPENAI_API_KEY"))
	}

	return ConfigDiffResult{Files: files}, nil
}

// copyJSONMap 深拷贝一个 JSON map，避免修改原始数据。

func copyJSONMap(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	b, _ := json.Marshal(data)
	var result map[string]any
	_ = json.Unmarshal(b, &result)
	return result
}

func (s *Service) previewApplyOpenCode(req ApplyAgentConfigRequest, port int, accessKey string) (ConfigDiffResult, error) {
	providerID, providerLabel, targetURL, apiKey, _, err := resolveOpenCodeProvider(req, port, accessKey)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	configPath := s.openCodeConfigPath()
	authPath := s.openCodeAuthPath()
	configContent, _, err := readTextFile(configPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	authData, _, err := readJSONMap(authPath)
	if err != nil {
		return ConfigDiffResult{}, err
	}
	providerJSON := openCodeProviderBlockJSON(providerID, providerLabel, targetURL)
	updatedConfig := patchOpenCodeProviderJSONC(configContent, providerID, providerJSON)
	newAuth := copyJSONMap(authData)
	newAuth = upsertOpenCodeAuthKey(newAuth, providerID, apiKey)
	return ConfigDiffResult{Files: []FileDiff{
		computeTextDiff(configPath, configContent, updatedConfig),
		computeJSONDiffWithMask(authPath, authData, newAuth, "key"),
	}}, nil
}

func (s *Service) previewRestoreOpenCode() (ConfigDiffResult, error) {
	var state OpenCodeProxyState
	if err := readJSONFile(s.openCodeStatePath(), &state); err != nil {
		return ConfigDiffResult{}, fmt.Errorf("未找到 OpenCode 配置状态，请先应用配置")
	}
	var files []FileDiff
	if state.ConfigFileExisted {
		content, _, err := readTextFile(state.ConfigPath)
		if err != nil {
			return ConfigDiffResult{}, err
		}
		var restored string
		if state.OriginalProviderJSON != nil {
			restored = patchOpenCodeProviderJSONC(content, state.ProviderID, *state.OriginalProviderJSON)
		} else {
			restored = removeJSONCObjectKey(content, state.ProviderID)
		}
		files = append(files, computeTextDiff(state.ConfigPath, content, restored))
	} else {
		content, _, _ := readTextFile(state.ConfigPath)
		files = append(files, computeTextDiff(state.ConfigPath, content, ""))
	}
	if state.AuthFileExisted {
		authData, _, err := readJSONMap(state.AuthPath)
		if err != nil {
			return ConfigDiffResult{}, err
		}
		restoredAuth := copyJSONMap(authData)
		restoredAuth = restoreOpenCodeAuthKey(restoredAuth, state.ProviderID, state.OriginalAuthType, state.OriginalAuthKey)
		files = append(files, computeJSONDiffWithMask(state.AuthPath, authData, restoredAuth, "key"))
	} else {
		authData, _, _ := readJSONMap(state.AuthPath)
		files = append(files, computeJSONDiffWithMask(state.AuthPath, authData, nil, "key"))
	}
	return ConfigDiffResult{Files: files}, nil
}
