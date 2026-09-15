package testutil

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
)

// LogCapture is a thread-safe slog sink for assertions.
type LogCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Handler returns a JSON slog handler writing into the capture at Debug level.
func (c *LogCapture) Handler() slog.Handler {
	return slog.NewJSONHandler(c, &slog.HandlerOptions{Level: slog.LevelDebug})
}

// Write implements io.Writer.
func (c *LogCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

// String returns everything captured so far.
func (c *LogCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// Lines returns each captured line decoded as JSON.
func (c *LogCapture) Lines() []map[string]any {
	var out []map[string]any
	for _, l := range strings.Split(c.String(), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err == nil {
			out = append(out, m)
		}
	}
	return out
}

// Count returns how many captured lines have field==value.
func (c *LogCapture) Count(field, value string) int {
	n := 0
	for _, l := range c.Lines() {
		if v, ok := l[field]; ok && v == value {
			n++
		}
	}
	return n
}
