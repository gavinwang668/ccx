package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCredentialUIDStableWithinAccount(t *testing.T) {
	first := GenerateCredentialUID("acct_test", "sk-test")
	second := GenerateCredentialUID("acct_test", "sk-test")
	if first == "" || first != second {
		t.Fatalf("credential uid 不稳定: first=%q second=%q", first, second)
	}
	if other := GenerateCredentialUID("acct_other", "sk-test"); other == first {
		t.Fatalf("不同账号不应共享 credential uid: %q", other)
	}
}

func TestUpdateAccountChannelsUpdatesAllRoutes(t *testing.T) {
	cm := &ConfigManager{config: Config{
		Upstream:     []UpstreamConfig{{AccountUID: "acct_test", ChannelUID: "ch_messages", ServiceType: "claude", ProviderID: "mimo", AutoManaged: true}},
		ChatUpstream: []UpstreamConfig{{AccountUID: "acct_test", ChannelUID: "ch_chat", ServiceType: "openai", ProviderID: "mimo", AutoManaged: true}},
	}}
	updates := []AccountChannelUpdate{
		{ChannelUID: "ch_messages", Name: "mimo-claude", APIKeys: []string{"sk-a", "sk-b"}, APIKeyConfig: []APIKeyConfig{{Key: "sk-a", BaseURL: "https://m.example/anthropic"}, {Key: "sk-b", BaseURL: "https://m.example/anthropic"}}, BaseURLs: []string{"https://m.example/anthropic"}},
		{ChannelUID: "ch_chat", Name: "mimo-chat", APIKeys: []string{"sk-a", "sk-b"}, APIKeyConfig: []APIKeyConfig{{Key: "sk-a", BaseURL: "https://m.example/v1"}, {Key: "sk-b", BaseURL: "https://m.example/v1"}}, BaseURLs: []string{"https://m.example/v1"}},
	}
	// 测试不落盘，只验证更新主体；临时配置文件让 saveConfigLocked 可正常写入。
	dir := t.TempDir()
	cm.configFile = dir + "/config.json"
	cm.backupDir = dir + "/backups"
	if err := cm.UpdateAccountChannels("acct_test", updates); err != nil {
		t.Fatalf("UpdateAccountChannels 失败: %v", err)
	}
	if len(cm.config.Upstream[0].APIKeys) != 2 || len(cm.config.ChatUpstream[0].APIKeys) != 2 {
		t.Fatalf("账号 Key 未同步到全部 route")
	}
	messageCred := cm.config.Upstream[0].APIKeyConfigs[0].CredentialUID
	chatCred := cm.config.ChatUpstream[0].APIKeyConfigs[0].CredentialUID
	if messageCred == "" || messageCred != chatCred {
		t.Fatalf("同账号同 Key 应共享 credential uid: messages=%q chat=%q", messageCred, chatCred)
	}
	data, err := os.ReadFile(cm.configFile)
	if err != nil {
		t.Fatalf("读取持久化配置失败: %v", err)
	}
	if count := strings.Count(string(data), "sk-a"); count != 1 {
		t.Fatalf("账号级 Key 应只持久化一次，sk-a 出现 %d 次", count)
	}
	var persisted Config
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("解析持久化配置失败: %v", err)
	}
	persisted.hydrateManagedAccountCredentials()
	if len(persisted.Upstream[0].APIKeys) != 2 || len(persisted.ChatUpstream[0].APIKeys) != 2 {
		t.Fatalf("加载时未从账号凭证恢复 route 运行时 Key")
	}
	removed, err := cm.DeleteAccountChannels("acct_test")
	if err != nil || len(removed) != 2 {
		t.Fatalf("DeleteAccountChannels removed=%v err=%v", removed, err)
	}
	if len(cm.config.Upstream) != 0 || len(cm.config.ChatUpstream) != 0 || len(cm.config.ManagedAccounts) != 0 {
		t.Fatalf("账号级删除未清理全部 route 或凭证源")
	}
}
