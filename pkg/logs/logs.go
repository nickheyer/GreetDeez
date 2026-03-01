package logs

import (
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Tees slog output to both stderr and an in-memory buffer
type LogCapture struct {
	mu    sync.Mutex
	lines []string
}

func (c *LogCapture) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\n")
	if line != "" {
		c.mu.Lock()
		c.lines = append(c.lines, line)
		if len(c.lines) > 1000 {
			c.lines = c.lines[len(c.lines)-1000:]
		}
		c.mu.Unlock()
	}
	return os.Stderr.Write(p)
}

func (c *LogCapture) Lines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.lines))
	copy(out, c.lines)
	return out
}

func InitLogger(devMode bool) *LogCapture {
	capture := &LogCapture{}
	level := slog.LevelInfo
	if devMode {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(capture, &slog.HandlerOptions{
		Level: level,
	})))
	return capture
}
