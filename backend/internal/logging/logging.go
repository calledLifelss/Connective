// Package logging is Connective's structured log subsystem.
//
// Levels: DEBUG < INFO < WARN < ERROR. Secrets (uuid, password, tokens)
// must never reach logs or the UI: callers pass values through Redact or
// use the Logger's secret-aware formatting. The in-app log viewer reads
// from the ring buffer exposed here.
package logging

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Level is a log severity.
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Entry is one log record.
type Entry struct {
	At      time.Time `json:"at"`
	Level   Level     `json:"level"`
	Message string    `json:"message"`
}

// sensitiveKeys are matched case-insensitively against message content for
// best-effort redaction of accidentally included secrets.
var sensitiveMarkers = []string{"uuid=", "password=", "passwd=", "token=", "pbk="}

// Redact replaces likely secret values in free text. It is a safety net,
// not a substitute for never logging secrets in the first place.
func Redact(msg string) string {
	for _, marker := range sensitiveMarkers {
		msg = redactMarker(msg, marker)
	}
	return msg
}

func redactMarker(msg, marker string) string {
	lower := strings.ToLower(msg)
	var out strings.Builder
	for {
		i := strings.Index(lower, marker)
		if i < 0 {
			out.WriteString(msg)
			break
		}
		out.WriteString(msg[:i+len(marker)])
		rest := msg[i+len(marker):]
		end := strings.IndexAny(rest, " \t\n\r\"',;")
		if end < 0 {
			out.WriteString("***")
			break
		}
		out.WriteString("***")
		msg, lower = rest[end:], strings.ToLower(rest[end:])
	}
	return out.String()
}

// Logger is a goroutine-safe leveled logger with a bounded ring buffer.
type Logger struct {
	mu      sync.Mutex
	level   Level
	entries []Entry
	max     int
	sink    func(Entry)
}

// New returns a Logger keeping at most max buffered entries.
func New(level Level, max int) *Logger {
	if max <= 0 {
		max = 2000
	}
	return &Logger{level: level, max: max}
}

// SetSink adds an extra output (e.g. file writer, IPC broadcaster).
func (l *Logger) SetSink(fn func(Entry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sink = fn
}

func (l *Logger) log(level Level, format string, args ...any) {
	if level < l.level {
		return
	}
	e := Entry{At: time.Now(), Level: level, Message: Redact(fmt.Sprintf(format, args...))}
	l.mu.Lock()
	l.entries = append(l.entries, e)
	if len(l.entries) > l.max {
		l.entries = l.entries[len(l.entries)-l.max:]
	}
	sink := l.sink
	l.mu.Unlock()
	if sink != nil {
		sink(e)
	}
}

// Debug, Info, Warn, Error emit at the respective levels.
func (l *Logger) Debug(f string, a ...any) { l.log(DEBUG, f, a...) }
func (l *Logger) Info(f string, a ...any)  { l.log(INFO, f, a...) }
func (l *Logger) Warn(f string, a ...any)  { l.log(WARN, f, a...) }
func (l *Logger) Error(f string, a ...any) { l.log(ERROR, f, a...) }

// Recent returns up to n newest entries at or above minLevel, newest last.
func (l *Logger) Recent(n int, minLevel Level) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Entry
	for _, e := range l.entries {
		if e.Level >= minLevel {
			out = append(out, e)
		}
	}
	if n > 0 && len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

// Clear drops buffered entries.
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}
