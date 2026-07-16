package autopilot

import "testing"

func TestModelProfileStoreActiveViewsFilterWithoutDeletingHistory(t *testing.T) {
	db := newTestDB(t)
	store, err := NewModelProfileStoreWithDB(db)
	if err != nil {
		t.Fatalf("NewModelProfileStoreWithDB 失败: %v", err)
	}

	active := &ModelProfile{
		ChannelUID:  "ch-a",
		ChannelKind: "messages",
		MetricsKey:  "mk-active",
		ModelID:     "claude-sonnet-5",
	}
	stale := &ModelProfile{
		ChannelUID:  "ch-a",
		ChannelKind: "messages",
		MetricsKey:  "mk-stale",
		ModelID:     "claude-opus-4-8",
	}
	for _, profile := range []*ModelProfile{active, stale} {
		if err := store.Upsert(profile); err != nil {
			t.Fatalf("Upsert 失败: %v", err)
		}
	}

	// 清单初始化前保持 fail-open。
	if got := len(store.ListActiveByChannel("ch-a")); got != 2 {
		t.Fatalf("初始化前有效模型画像数=%d, want 2", got)
	}
	store.ReplaceActiveBindings(map[string]struct{}{
		modelProfileBindingKey(active.ChannelUID, active.ChannelKind, active.MetricsKey): {},
	})

	if got := len(store.ListActiveByChannel("ch-a")); got != 1 {
		t.Fatalf("有效模型画像数=%d, want 1", got)
	}
	if got := len(store.GetModelProfiles("ch-a", "messages", "mk-active")); got != 1 {
		t.Fatalf("有效 binding 查询数=%d, want 1", got)
	}
	if got := len(store.GetModelProfiles("ch-a", "messages", "mk-stale")); got != 0 {
		t.Fatalf("失效 binding 查询数=%d, want 0", got)
	}
	if got := len(store.ListByChannel("ch-a")); got != 2 {
		t.Fatalf("历史模型画像被删除: ListByChannel=%d, want 2", got)
	}
	if store.Get("ch-a", "messages", "mk-stale", stale.ModelID) == nil {
		t.Fatal("失效模型画像仍应保留，供审计或显式清理")
	}

	store.ReplaceActiveBindings(nil)
	if got := len(store.ListActiveByChannel("ch-a")); got != 0 {
		t.Fatalf("空配置下有效模型画像数=%d, want 0", got)
	}
	if got := len(store.ListByChannel("ch-a")); got != 2 {
		t.Fatalf("空配置不应删除历史模型画像: ListByChannel=%d, want 2", got)
	}
}
