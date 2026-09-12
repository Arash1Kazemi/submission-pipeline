package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Health tracks whether th consumer loop is still alive.
// The loop calls MarkPoll after eatch queue poll;
// the endpoint reports unhealthy when that heartbeat gose stale.

type Health struct {
	mu       sync.RWMutex
	lastPoll time.Time
}

func (h *Health) MarkPoll() {
	h.mu.Lock()
	h.lastPoll = time.Now()
	h.mu.Unlock()
}

func (h *Health) Handle(maxAge time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.mu.RLock()
		age := time.Since(h.lastPoll)
		h.mu.RUnlock()

		if age > maxAge {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "stale: last queue poll %s ago\n", age.Round(time.Second))
			return
		}
		fmt.Fprintln(w, "ok")
	}
}
