package autopilot

import (
	"sync"
	"testing"
	"time"
)

func TestEventHub_SubscribeAndPublish(t *testing.T) {
	hub := NewEventHub()
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	hub.Publish(ProfileChangeEvent{ChannelUID: "ch-1", EventType: EventTypeHealthChanged})

	select {
	case ev := <-ch:
		if ev.ChannelUID != "ch-1" {
			t.Errorf("收到的事件不匹配: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("超时未收到事件")
	}
}

func TestEventHub_MultipleSubscribersAllReceive(t *testing.T) {
	hub := NewEventHub()
	ch1, unsub1 := hub.Subscribe()
	ch2, unsub2 := hub.Subscribe()
	defer unsub1()
	defer unsub2()

	hub.Publish(ProfileChangeEvent{ChannelUID: "ch-broadcast", EventType: EventTypeProfileUpdated})

	for i, ch := range []chan ProfileChangeEvent{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.ChannelUID != "ch-broadcast" {
				t.Errorf("订阅者 %d 收到的事件不匹配: %+v", i, ev)
			}
		case <-time.After(time.Second):
			t.Fatalf("订阅者 %d 超时未收到事件", i)
		}
	}
}

func TestEventHub_PublishWithNoSubscribers_NoOp(t *testing.T) {
	hub := NewEventHub()
	// 不应 panic 或阻塞
	hub.Publish(ProfileChangeEvent{ChannelUID: "ch-1"})
}

func TestEventHub_UnsubscribeStopsDelivery(t *testing.T) {
	hub := NewEventHub()
	ch, unsubscribe := hub.Subscribe()
	unsubscribe()

	hub.Publish(ProfileChangeEvent{ChannelUID: "ch-1"})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("取消订阅后不应再收到事件")
		}
		// channel 已关闭，收到零值 + ok=false 是预期行为
	case <-time.After(200 * time.Millisecond):
		t.Fatal("取消订阅后 channel 应已关闭，读取不应阻塞")
	}
}

func TestEventHub_UnsubscribeIsIdempotent(t *testing.T) {
	hub := NewEventHub()
	_, unsubscribe := hub.Subscribe()
	unsubscribe()
	unsubscribe() // 不应 panic（重复关闭已关闭的 channel）
}

func TestEventHub_SlowSubscriberDoesNotBlockPublish(t *testing.T) {
	hub := NewEventHub()
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	// 灌满该订阅者的缓冲区，且不读取
	for i := 0; i < eventHubBufferSize+10; i++ {
		hub.Publish(ProfileChangeEvent{ChannelUID: "ch-flood"})
	}
	// 只要能执行到这里说明 Publish 没有阻塞
	if len(ch) != eventHubBufferSize {
		t.Errorf("缓冲区应被填满至容量上限 %d, got %d", eventHubBufferSize, len(ch))
	}
}

func TestEventHub_ConcurrentSubscribeAndPublish(t *testing.T) {
	hub := NewEventHub()
	var wg sync.WaitGroup

	// 并发订阅/取消订阅
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unsubscribe := hub.Subscribe()
			defer unsubscribe()
			select {
			case <-ch:
			case <-time.After(50 * time.Millisecond):
			}
		}()
	}

	// 并发发布
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hub.Publish(ProfileChangeEvent{ChannelUID: "ch-concurrent"})
		}()
	}

	wg.Wait()
}

func TestEventHub_SubscriberCount(t *testing.T) {
	hub := NewEventHub()
	if hub.SubscriberCount() != 0 {
		t.Fatalf("初始应为 0，got %d", hub.SubscriberCount())
	}
	_, unsub1 := hub.Subscribe()
	_, unsub2 := hub.Subscribe()
	if hub.SubscriberCount() != 2 {
		t.Fatalf("应为 2，got %d", hub.SubscriberCount())
	}
	unsub1()
	if hub.SubscriberCount() != 1 {
		t.Fatalf("取消一个后应为 1，got %d", hub.SubscriberCount())
	}
	unsub2()
	if hub.SubscriberCount() != 0 {
		t.Fatalf("全部取消后应为 0，got %d", hub.SubscriberCount())
	}
}
