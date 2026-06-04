package lark_http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SSEWriter struct {
	ctx     *Context
	flusher http.Flusher
	mu      sync.Mutex
}

func (ctx *Context) SSE() *SSEWriter {
	flusher, _ := ctx.Writer.(http.Flusher)
	ctx.flushed = true
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	for k, v := range ctx.headers {
		ctx.Writer.Header().Set(k, v)
	}
	ctx.Writer.WriteHeader(http.StatusOK)
	return &SSEWriter{ctx: ctx, flusher: flusher}
}

func (w *SSEWriter) Event(event string, data string) {
	w.mu.Lock()
	fmt.Fprintf(w.ctx.Writer, "event: %s\n", event)
	w.writeData(data)
	w.flush()
	w.mu.Unlock()
}

func (w *SSEWriter) Data(data string) {
	w.mu.Lock()
	w.writeData(data)
	w.flush()
	w.mu.Unlock()
}

func (w *SSEWriter) JSON(event string, obj interface{}) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return
	}
	w.mu.Lock()
	if event != "" {
		fmt.Fprintf(w.ctx.Writer, "event: %s\n", event)
	}
	fmt.Fprintf(w.ctx.Writer, "data: %s\n\n", bytes)
	w.flush()
	w.mu.Unlock()
}

func (w *SSEWriter) ID(id string) {
	w.mu.Lock()
	fmt.Fprintf(w.ctx.Writer, "id: %s\n", id)
	w.flush()
	w.mu.Unlock()
}

func (w *SSEWriter) Retry(ms int) {
	w.mu.Lock()
	fmt.Fprintf(w.ctx.Writer, "retry: %d\n\n", ms)
	w.flush()
	w.mu.Unlock()
}

func (w *SSEWriter) Comment(text string) {
	w.mu.Lock()
	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintf(w.ctx.Writer, ": %s\n", line)
	}
	fmt.Fprint(w.ctx.Writer, "\n")
	w.flush()
	w.mu.Unlock()
}

func (w *SSEWriter) Heartbeat(interval time.Duration) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.mu.Lock()
				fmt.Fprint(w.ctx.Writer, ": keepalive\n\n")
				w.flush()
				w.mu.Unlock()
			case <-w.ctx.Request.Context().Done():
				return
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

func (w *SSEWriter) Closed() <-chan struct{} {
	return w.ctx.Request.Context().Done()
}

func (w *SSEWriter) Stream(ch <-chan string) {
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			w.Data(msg)
		case <-w.ctx.Request.Context().Done():
			return
		}
	}
}

func (w *SSEWriter) Flush() {
	w.flush()
}

func (w *SSEWriter) Done() {
	w.Data("[DONE]")
}

func (w *SSEWriter) writeData(data string) {
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		fmt.Fprintf(w.ctx.Writer, "data: %s\n", line)
	}
	fmt.Fprint(w.ctx.Writer, "\n")
}

func (w *SSEWriter) flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}
