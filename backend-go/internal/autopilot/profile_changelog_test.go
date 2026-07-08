package autopilot

import (
	"testing"
	"time"
)

func newTestChangelogStore(t *testing.T) *ProfileChangelogStore {
	t.Helper()
	db := newTestDB(t)
	store, err := NewProfileChangelogStoreWithDB(db)
	if err != nil {
		t.Fatalf("创建 ProfileChangelogStore 失败: %v", err)
	}
	return store
}

func TestProfileChangelogStore_RecordAndListRecent(t *testing.T) {
	store := newTestChangelogStore(t)

	store.Record(ProfileChangeEvent{
		ChannelUID:  "ch-1",
		ChannelKind: "messages",
		EndpointUID: "ep-1",
		EventType:   EventTypeHealthChanged,
		Summary:     "healthy → degraded",
	})
	store.Record(ProfileChangeEvent{
		ChannelUID:  "ch-2",
		ChannelKind: "chat",
		EndpointUID: "ep-2",
		EventType:   EventTypeProfileUpdated,
		Summary:     "quality: normal → high",
	})

	recent := store.ListRecent(10)
	if len(recent) != 2 {
		t.Fatalf("ListRecent 应返回 2 条，got %d", len(recent))
	}
	// 时间降序：最近写入的在最前
	if recent[0].ChannelUID != "ch-2" {
		t.Errorf("最新记录应在最前: got channelUid=%s", recent[0].ChannelUID)
	}
	if recent[0].EventUID == "" {
		t.Error("EventUID 应被自动填充")
	}
	if recent[0].CreatedAt.IsZero() {
		t.Error("CreatedAt 应被自动填充")
	}
}

func TestProfileChangelogStore_ListRecent_LimitExceedsTotal(t *testing.T) {
	store := newTestChangelogStore(t)
	store.Record(ProfileChangeEvent{ChannelUID: "ch-1", EndpointUID: "ep-1", EventType: EventTypeHealthChanged})

	recent := store.ListRecent(100)
	if len(recent) != 1 {
		t.Fatalf("请求数超过总量应返回全部，got %d", len(recent))
	}
}

func TestProfileChangelogStore_ListByChannel_FiltersCorrectly(t *testing.T) {
	store := newTestChangelogStore(t)
	store.Record(ProfileChangeEvent{ChannelUID: "ch-1", EndpointUID: "ep-1", EventType: EventTypeHealthChanged})
	store.Record(ProfileChangeEvent{ChannelUID: "ch-2", EndpointUID: "ep-2", EventType: EventTypeHealthChanged})
	store.Record(ProfileChangeEvent{ChannelUID: "ch-1", EndpointUID: "ep-3", EventType: EventTypeProfileUpdated})

	byChannel := store.ListByChannel("ch-1", 10)
	if len(byChannel) != 2 {
		t.Fatalf("ch-1 应有 2 条记录，got %d", len(byChannel))
	}
	for _, ev := range byChannel {
		if ev.ChannelUID != "ch-1" {
			t.Errorf("过滤结果不应包含其他渠道: %+v", ev)
		}
	}
}

func TestProfileChangelogStore_RingBufferEviction(t *testing.T) {
	store := newTestChangelogStore(t)
	for i := 0; i < changelogMaxRecords+10; i++ {
		store.Record(ProfileChangeEvent{
			ChannelUID:  "ch-1",
			EndpointUID: "ep-1",
			EventType:   EventTypeHealthChanged,
			CreatedAt:   time.Now().Add(time.Duration(i) * time.Millisecond),
		})
	}
	recent := store.ListRecent(0)
	if len(recent) != changelogMaxRecords {
		t.Fatalf("内存环形应封顶在 %d 条，got %d", changelogMaxRecords, len(recent))
	}
}

func TestProfileChangelogStore_PersistedAcrossReopen(t *testing.T) {
	db := newTestDB(t)
	store1, err := NewProfileChangelogStoreWithDB(db)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	store1.Record(ProfileChangeEvent{
		ChannelUID:  "ch-persist",
		ChannelKind: "messages",
		EndpointUID: "ep-persist",
		EventType:   EventTypeHealthChanged,
		Summary:     "healthy → dead",
	})

	// 用同一个 db 重新打开一个 store，模拟服务重启后从 SQLite 加载历史
	store2, err := NewProfileChangelogStoreWithDB(db)
	if err != nil {
		t.Fatalf("重新打开失败: %v", err)
	}
	recent := store2.ListRecent(10)
	if len(recent) != 1 {
		t.Fatalf("重启后应加载到 1 条历史记录，got %d", len(recent))
	}
	if recent[0].ChannelUID != "ch-persist" {
		t.Errorf("重启后 ChannelUID 不匹配: got=%s", recent[0].ChannelUID)
	}
}

func TestProfileChangelogStore_PruneExpired(t *testing.T) {
	store := newTestChangelogStore(t)

	old := time.Now().UTC().Add(-40 * 24 * time.Hour)
	fresh := time.Now().UTC()

	store.Record(ProfileChangeEvent{
		ChannelUID:  "ch-old",
		EndpointUID: "ep-old",
		EventType:   EventTypeHealthChanged,
		CreatedAt:   old,
	})
	store.Record(ProfileChangeEvent{
		ChannelUID:  "ch-fresh",
		EndpointUID: "ep-fresh",
		EventType:   EventTypeHealthChanged,
		CreatedAt:   fresh,
	})

	if err := store.pruneExpired(); err != nil {
		t.Fatalf("pruneExpired 失败: %v", err)
	}

	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM profile_changelog`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("清理后 SQLite 应只剩 1 条（fresh），got %d", count)
	}
}

func TestProfileChangelogStore_NilDB_MemoryOnly(t *testing.T) {
	store, err := NewProfileChangelogStoreWithDB(nil)
	if err != nil {
		t.Fatalf("nil db 不应报错: %v", err)
	}
	store.Record(ProfileChangeEvent{ChannelUID: "ch-1", EndpointUID: "ep-1", EventType: EventTypeHealthChanged})
	if len(store.ListRecent(10)) != 1 {
		t.Fatal("nil db 时仍应支持内存读写")
	}
}
