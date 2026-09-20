// Package ssehub 维护"账号 -> 实时连接"的内存表，负责把通知事件实时推送给在线的收件人。
// SSE 长连接只可能由 api 进程持有（HTTP 服务），故本包仅在 api 进程装配使用。
package ssehub

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 每连接出站缓冲；满则丢弃(实时推送尽力而为)，不阻塞业务写。
const sendBufSize = 16

// conn 单条 SSE 连接
type conn struct {
	send chan []byte // 出站消息(待写 SSG data)
}

// Hub 内存连接表：recipientID -> 连接集合
type Hub struct {
	mu    sync.RWMutex
	conns map[uint]map[*conn]struct{}
}

// NewHub 创建连接表
func NewHub() *Hub {
	return &Hub{conns: make(map[uint]map[*conn]struct{})}
}

// add 注册收件人的一条连接
func (h *Hub) add(recipientID uint, c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[recipientID] == nil {
		h.conns[recipientID] = make(map[*conn]struct{})
	}
	h.conns[recipientID][c] = struct{}{}
}

// remove 注销收件人的一条连接
func (h *Hub) remove(recipientID uint, c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.conns[recipientID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.conns, recipientID)
		}
	}
}

// Push 定向推送给收件人的所有在线连接；无在线连接时静默跳过。
func (h *Hub) Push(recipientID uint, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.conns[recipientID] {
		select {
		case c.send <- payload:
		default:
			// 发送缓冲满：丢该条，保证不阻塞推送方
		}
	}
}

// ServeSSE 以 text/event-stream 为当前账号建立实时通知流。
// recipientID 由已通过 JWT 鉴权的调用方传入。
func (h *Hub) ServeSSE(recipientID uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")

		// 先刷新头，让客户端立即收到 200 + 流头
		c.Writer.Flush()

		cl := &conn{send: make(chan []byte, sendBufSize)}
		h.add(recipientID, cl)
		defer h.remove(recipientID, cl)

		// 心跳 tick：SSE 规范空行冒号注释行防代理超时
		keepalive := time.NewTicker(25 * time.Second)
		defer keepalive.Stop()

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			return
		}

		disconnected := func() bool {
			select {
			case <-ctx.Done():
				return true
			default:
				return false
			}
		}

		for {
			if disconnected() {
				return
			}
			select {
			case <-ctx.Done():
				return
			case payload := <-cl.send:
				if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", payload); err != nil {
					return
				}
				flusher.Flush()
			case <-keepalive.C:
				if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}
