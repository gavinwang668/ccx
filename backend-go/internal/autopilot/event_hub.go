package autopilot

import "sync"

// ── EventHub（Phase 3A：画像变更事件内存 pub/sub）──
//
// 纯内存 fan-out，不依赖外部消息队列。用于把 Manager 后台 worker 检测到的
// ProfileChangeEvent 广播给前端 WebSocket 连接。没有订阅者时 Publish 是
// O(0) 的空操作，慢订阅者不会阻塞发布方（非阻塞发送，塞满就丢弃该条给该订阅者）。

// eventHubBufferSize 每个订阅者的缓冲区容量。
// 变更事件本身是低频信号（健康状态/能力标签变化 + discovery/mapping 完成），
// 缓冲区只需容纳短暂的网络抖动，不需要很大。
const eventHubBufferSize = 32

// EventHub 管理 ProfileChangeEvent 的订阅与广播。
type EventHub struct {
	mu   sync.RWMutex
	subs map[chan ProfileChangeEvent]struct{}
}

// NewEventHub 创建 EventHub。
func NewEventHub() *EventHub {
	return &EventHub{
		subs: make(map[chan ProfileChangeEvent]struct{}),
	}
}

// Subscribe 注册一个新订阅者，返回接收 channel 和取消订阅函数。
// unsubscribe 可安全多次调用；调用后该 channel 会被关闭。
func (h *EventHub) Subscribe() (ch chan ProfileChangeEvent, unsubscribe func()) {
	ch = make(chan ProfileChangeEvent, eventHubBufferSize)

	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	unsubscribe = func() {
		once.Do(func() {
			h.mu.Lock()
			if _, ok := h.subs[ch]; ok {
				delete(h.subs, ch)
				close(ch)
			}
			h.mu.Unlock()
		})
	}
	return ch, unsubscribe
}

// Publish 向所有当前订阅者广播一个事件。非阻塞：订阅者缓冲区已满时丢弃该条
// （不影响其他订阅者，也不阻塞调用方，例如后台 worker 的 collectAll 循环）。
func (h *EventHub) Publish(ev ProfileChangeEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subs {
		select {
		case ch <- ev:
		default:
			// 订阅者消费太慢，丢弃本条事件，不阻塞发布方。
		}
	}
}

// SubscriberCount 返回当前订阅者数量，主要用于测试与诊断。
func (h *EventHub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}
