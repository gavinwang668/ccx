package autopilot

import (
	"testing"
)

func newTestManagerForEmit(t *testing.T) *Manager {
	t.Helper()
	db := newTestDB(t)
	store, err := NewProfileStoreWithDB(db)
	if err != nil {
		t.Fatalf("创建 ProfileStore 失败: %v", err)
	}
	changelogStore, err := NewProfileChangelogStoreWithDB(db)
	if err != nil {
		t.Fatalf("创建 ProfileChangelogStore 失败: %v", err)
	}
	return &Manager{
		store:          store,
		changelogStore: changelogStore,
		eventHub:       NewEventHub(),
	}
}

func TestManager_EmitProfileChangeEvents_FirstProfileNoEvents(t *testing.T) {
	mgr := newTestManagerForEmit(t)

	current := newTestProfile("ep-1", "ch-1", "messages", "https://example.com")
	current.HealthState = HealthStateHealthy

	// store 中尚无旧值（首次画像）
	mgr.emitProfileChangeEvents(current.EndpointUID, current, false)

	if got := mgr.changelogStore.ListRecent(10); len(got) != 0 {
		t.Fatalf("首次画像不应产出变更事件，got %d", len(got))
	}
}

func TestManager_EmitProfileChangeEvents_HealthChangeRecordsAndPublishes(t *testing.T) {
	mgr := newTestManagerForEmit(t)

	old := newTestProfile("ep-1", "ch-1", "messages", "https://example.com")
	old.HealthState = HealthStateHealthy
	if err := mgr.store.Upsert(old); err != nil {
		t.Fatalf("写入旧画像失败: %v", err)
	}

	sub, unsubscribe := mgr.eventHub.Subscribe()
	defer unsubscribe()

	current := newTestProfile("ep-1", "ch-1", "messages", "https://example.com")
	current.HealthState = HealthStateDegraded

	mgr.emitProfileChangeEvents(current.EndpointUID, current, false)

	recorded := mgr.changelogStore.ListRecent(10)
	if len(recorded) != 1 {
		t.Fatalf("HealthState 变化应写入 1 条 changelog，got %d", len(recorded))
	}
	if recorded[0].EventType != EventTypeHealthChanged {
		t.Errorf("EventType = %q, want %q", recorded[0].EventType, EventTypeHealthChanged)
	}

	select {
	case ev := <-sub:
		if ev.EventType != EventTypeHealthChanged {
			t.Errorf("hub 收到的 EventType = %q, want %q", ev.EventType, EventTypeHealthChanged)
		}
	default:
		t.Fatal("EventHub 应该已收到广播事件")
	}
}

func TestManager_EmitProfileChangeEvents_NoChangeProducesNothing(t *testing.T) {
	mgr := newTestManagerForEmit(t)

	profile := newTestProfile("ep-1", "ch-1", "messages", "https://example.com")
	profile.HealthState = HealthStateHealthy
	if err := mgr.store.Upsert(profile); err != nil {
		t.Fatalf("写入旧画像失败: %v", err)
	}

	same := newTestProfile("ep-1", "ch-1", "messages", "https://example.com")
	same.HealthState = HealthStateHealthy

	mgr.emitProfileChangeEvents(same.EndpointUID, same, false)

	if got := mgr.changelogStore.ListRecent(10); len(got) != 0 {
		t.Fatalf("画像不变不应产出事件，got %d", len(got))
	}
}

func TestManager_EmitProfileChangeEvents_NilStoresAreNoop(t *testing.T) {
	db := newTestDB(t)
	store, err := NewProfileStoreWithDB(db)
	if err != nil {
		t.Fatalf("创建 ProfileStore 失败: %v", err)
	}
	mgr := &Manager{store: store} // changelogStore/eventHub 均为 nil

	profile := newTestProfile("ep-1", "ch-1", "messages", "https://example.com")
	// 不应 panic
	mgr.emitProfileChangeEvents(profile.EndpointUID, profile, false)
}
