package config

import (
	"fmt"
	"strings"
)

// AccountChannel 是账号级管理 API 使用的渠道快照。
type AccountChannel struct {
	Kind     string
	Upstream UpstreamConfig
}

// AccountChannelUpdate 描述一次账号更新中单条协议渠道的新凭证绑定。
type AccountChannelUpdate struct {
	ChannelUID   string
	Name         string
	APIKeys      []string
	APIKeyConfig []APIKeyConfig
	BaseURLs     []string
}

// GetAccountChannels 返回账号下全部协议渠道的深拷贝。
func (cm *ConfigManager) GetAccountChannels(accountUID string) []AccountChannel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []AccountChannel
	visit := func(kind string, channels []UpstreamConfig) {
		for i := range channels {
			if channels[i].AccountUID != accountUID {
				continue
			}
			result = append(result, AccountChannel{Kind: kind, Upstream: *channels[i].Clone()})
		}
	}
	visit("messages", cm.config.Upstream)
	visit("chat", cm.config.ChatUpstream)
	visit("responses", cm.config.ResponsesUpstream)
	visit("gemini", cm.config.GeminiUpstream)
	visit("images", cm.config.ImagesUpstream)
	visit("vectors", cm.config.VectorsUpstream)
	return result
}

// UpdateAccountChannels 原子更新账号下所有协议渠道的 Key -> BaseURL 绑定。
func (cm *ConfigManager) UpdateAccountChannels(accountUID string, updates []AccountChannelUpdate) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	byChannel := make(map[string]AccountChannelUpdate, len(updates))
	for _, update := range updates {
		byChannel[update.ChannelUID] = update
	}
	known := 0
	total := 0
	countKnown := func(channels []UpstreamConfig) {
		for i := range channels {
			if channels[i].AccountUID == accountUID {
				total++
				if _, ok := byChannel[channels[i].ChannelUID]; ok {
					known++
				}
			}
		}
	}
	countKnown(cm.config.Upstream)
	countKnown(cm.config.ChatUpstream)
	countKnown(cm.config.ResponsesUpstream)
	countKnown(cm.config.GeminiUpstream)
	countKnown(cm.config.ImagesUpstream)
	countKnown(cm.config.VectorsUpstream)
	if known == 0 {
		return fmt.Errorf("账号 %s 不存在或没有可更新渠道", accountUID)
	}
	if known != total || len(updates) != total {
		return fmt.Errorf("账号 %s 渠道更新不完整: matched=%d total=%d updates=%d", accountUID, known, total, len(updates))
	}

	matched := 0
	apply := func(channels []UpstreamConfig) {
		for i := range channels {
			channel := &channels[i]
			if channel.AccountUID != accountUID {
				continue
			}
			update, ok := byChannel[channel.ChannelUID]
			if !ok {
				continue
			}
			channel.Name = update.Name
			channel.APIKeys = deduplicateStrings(update.APIKeys)
			channel.APIKeyConfigs = normalizeAPIKeyConfigs(channel.APIKeys, update.APIKeyConfig)
			for j := range channel.APIKeyConfigs {
				if channel.APIKeyConfigs[j].CredentialUID == "" {
					channel.APIKeyConfigs[j].CredentialUID = GenerateCredentialUID(accountUID, channel.APIKeyConfigs[j].Key)
				}
			}
			channel.BaseURLs = deduplicateBaseURLs(update.BaseURLs, channel.ServiceType)
			if len(channel.BaseURLs) > 0 {
				channel.BaseURL = channel.BaseURLs[0]
			}
			if len(channel.APIKeys) > 0 && channel.Status == "suspended" {
				channel.Status = "active"
			}
			matched++
		}
	}
	apply(cm.config.Upstream)
	apply(cm.config.ChatUpstream)
	apply(cm.config.ResponsesUpstream)
	apply(cm.config.GeminiUpstream)
	apply(cm.config.ImagesUpstream)
	apply(cm.config.VectorsUpstream)

	if matched != known {
		return fmt.Errorf("账号 %s 渠道更新计数异常: matched=%d known=%d", accountUID, matched, known)
	}
	for i := range cm.config.ManagedAccounts {
		if cm.config.ManagedAccounts[i].AccountUID == accountUID && len(updates) > 0 {
			cm.config.ManagedAccounts[i].Name = managedAccountName(updates[0].Name)
		}
	}
	return cm.saveConfigLocked(cm.config)
}

// DeleteAccountChannels 原子删除账号下全部协议渠道，返回被删除的 channelUid。
func (cm *ConfigManager) DeleteAccountChannels(accountUID string) ([]string, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var removed []string
	filter := func(channels []UpstreamConfig) []UpstreamConfig {
		kept := channels[:0]
		for _, channel := range channels {
			if channel.AccountUID == accountUID {
				removed = append(removed, channel.ChannelUID)
				continue
			}
			kept = append(kept, channel)
		}
		return kept
	}
	cm.config.Upstream = filter(cm.config.Upstream)
	cm.config.ChatUpstream = filter(cm.config.ChatUpstream)
	cm.config.ResponsesUpstream = filter(cm.config.ResponsesUpstream)
	cm.config.GeminiUpstream = filter(cm.config.GeminiUpstream)
	cm.config.ImagesUpstream = filter(cm.config.ImagesUpstream)
	cm.config.VectorsUpstream = filter(cm.config.VectorsUpstream)
	if len(removed) == 0 {
		return nil, fmt.Errorf("账号 %s 不存在", accountUID)
	}
	accounts := cm.config.ManagedAccounts[:0]
	for _, account := range cm.config.ManagedAccounts {
		if account.AccountUID != accountUID {
			accounts = append(accounts, account)
		}
	}
	cm.config.ManagedAccounts = accounts
	if err := cm.saveConfigLocked(cm.config); err != nil {
		return nil, err
	}
	return removed, nil
}

// RenameManagedAccount 原子重命名账号及其全部协议渠道。
func (cm *ConfigManager) RenameManagedAccount(accountUID, baseName string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		return fmt.Errorf("账号名称不能为空")
	}
	total := 0
	count := func(channels []UpstreamConfig) {
		for i := range channels {
			if channels[i].AccountUID == accountUID {
				total++
			}
		}
	}
	count(cm.config.Upstream)
	count(cm.config.ChatUpstream)
	count(cm.config.ResponsesUpstream)
	count(cm.config.GeminiUpstream)
	count(cm.config.ImagesUpstream)
	count(cm.config.VectorsUpstream)
	matched := 0
	rename := func(kind string, channels []UpstreamConfig) {
		for i := range channels {
			if channels[i].AccountUID == accountUID {
				channels[i].Name = baseName
				if total > 1 {
					channels[i].Name += accountChannelSuffix(kind)
				}
				matched++
			}
		}
	}
	rename("messages", cm.config.Upstream)
	rename("chat", cm.config.ChatUpstream)
	rename("responses", cm.config.ResponsesUpstream)
	rename("gemini", cm.config.GeminiUpstream)
	rename("images", cm.config.ImagesUpstream)
	rename("vectors", cm.config.VectorsUpstream)
	if matched == 0 {
		return fmt.Errorf("账号 %s 不存在", accountUID)
	}
	for i := range cm.config.ManagedAccounts {
		if cm.config.ManagedAccounts[i].AccountUID == accountUID {
			cm.config.ManagedAccounts[i].Name = baseName
		}
	}
	return cm.saveConfigLocked(cm.config)
}

func accountChannelSuffix(kind string) string {
	switch kind {
	case "messages":
		return "-claude"
	case "chat":
		return "-chat"
	case "responses":
		return "-codex"
	case "gemini":
		return "-gemini"
	default:
		return "-" + kind
	}
}

func (c *Config) syncManagedAccountsFromChannels() {
	existingOrder := append([]ManagedAccountConfig(nil), c.ManagedAccounts...)
	accounts := make(map[string]ManagedAccountConfig, len(c.ManagedAccounts))
	for _, account := range c.ManagedAccounts {
		account.Credentials = nil
		accounts[account.AccountUID] = account
	}
	credentialSeen := make(map[string]map[string]bool, len(accounts))
	visit := func(channels []UpstreamConfig) {
		for i := range channels {
			channel := &channels[i]
			if !channel.AutoManaged || channel.AccountUID == "" || channel.ProviderID == "" {
				continue
			}
			account := accounts[channel.AccountUID]
			account.AccountUID = channel.AccountUID
			account.ProviderID = channel.ProviderID
			if account.Name == "" {
				account.Name = managedAccountName(channel.Name)
			}
			seen := credentialSeen[channel.AccountUID]
			if seen == nil {
				seen = make(map[string]bool, len(channel.APIKeys))
				credentialSeen[channel.AccountUID] = seen
			}
			for _, apiKey := range channel.APIKeys {
				uid := channel.CredentialUIDForKey(apiKey)
				if !seen[uid] {
					account.Credentials = append(account.Credentials, ManagedAccountCredential{CredentialUID: uid, APIKey: apiKey})
					seen[uid] = true
				}
			}
			accounts[channel.AccountUID] = account
		}
	}
	visit(c.Upstream)
	visit(c.ChatUpstream)
	visit(c.ResponsesUpstream)
	visit(c.GeminiUpstream)
	visit(c.ImagesUpstream)
	visit(c.VectorsUpstream)
	c.ManagedAccounts = c.ManagedAccounts[:0]
	seen := make(map[string]bool, len(accounts))
	for _, existing := range existingOrder {
		if account, ok := accounts[existing.AccountUID]; ok {
			c.ManagedAccounts = append(c.ManagedAccounts, account)
			seen[existing.AccountUID] = true
		}
	}
	for uid, account := range accounts {
		if !seen[uid] {
			c.ManagedAccounts = append(c.ManagedAccounts, account)
		}
	}
}

func (c *Config) hydrateManagedAccountCredentials() {
	credentials := make(map[string]map[string]string, len(c.ManagedAccounts))
	for _, account := range c.ManagedAccounts {
		byUID := make(map[string]string, len(account.Credentials))
		for _, credential := range account.Credentials {
			byUID[credential.CredentialUID] = credential.APIKey
		}
		credentials[account.AccountUID] = byUID
	}
	visit := func(channels []UpstreamConfig) {
		for i := range channels {
			channel := &channels[i]
			byUID := credentials[channel.AccountUID]
			if len(byUID) == 0 {
				continue
			}
			channel.APIKeys = channel.APIKeys[:0]
			for j := range channel.APIKeyConfigs {
				if apiKey := byUID[channel.APIKeyConfigs[j].CredentialUID]; apiKey != "" {
					channel.APIKeyConfigs[j].Key = apiKey
					channel.APIKeys = append(channel.APIKeys, apiKey)
				}
			}
		}
	}
	visit(c.Upstream)
	visit(c.ChatUpstream)
	visit(c.ResponsesUpstream)
	visit(c.GeminiUpstream)
	visit(c.ImagesUpstream)
	visit(c.VectorsUpstream)
}

func (c *Config) stripManagedChannelSecrets() {
	visit := func(channels []UpstreamConfig) {
		for i := range channels {
			channel := &channels[i]
			if !channel.AutoManaged || channel.AccountUID == "" || channel.ProviderID == "" {
				continue
			}
			channel.APIKeys = nil
			for j := range channel.APIKeyConfigs {
				channel.APIKeyConfigs[j].Key = ""
			}
		}
	}
	visit(c.Upstream)
	visit(c.ChatUpstream)
	visit(c.ResponsesUpstream)
	visit(c.GeminiUpstream)
	visit(c.ImagesUpstream)
	visit(c.VectorsUpstream)
}

func managedAccountName(channelName string) string {
	for _, suffix := range []string{"-claude", "-chat", "-codex", "-gemini"} {
		channelName = strings.TrimSuffix(channelName, suffix)
	}
	return channelName
}
