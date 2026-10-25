package casbin

import (
	"log/slog"

	casbinlog "github.com/casbin/casbin/v3/log"
)

type casbinLogger struct {
	eventTypes map[casbinlog.EventType]struct{}
	callback   func(entry *casbinlog.LogEntry) error
}

func newCasbinLogger() casbinlog.Logger {
	return &casbinLogger{}
}

func (l *casbinLogger) SetEventTypes(types []casbinlog.EventType) error {
	l.eventTypes = make(map[casbinlog.EventType]struct{}, len(types))
	for _, eventType := range types {
		l.eventTypes[eventType] = struct{}{}
	}
	return nil
}

// OnBeforeEvent 是 casbinlog.Logger 接口要求的方法。
func (l *casbinLogger) OnBeforeEvent(entry *casbinlog.LogEntry) error {
	return nil
}

func (l *casbinLogger) OnAfterEvent(entry *casbinlog.LogEntry) error {
	if !l.shouldLog(entry.EventType) {
		return nil
	}

	logger := l.loggerForEntry(entry)
	switch {
	case entry.Error != nil:
		logger.Error("casbin event failed", "error", entry.Error)
	case entry.EventType == casbinlog.EventEnforce && !entry.Allowed:
		logger.Warn("casbin authorization denied", "allowed", false)
	}

	if l.callback != nil {
		return l.callback(entry)
	}
	return nil
}

func (l *casbinLogger) SetLogCallback(callback func(entry *casbinlog.LogEntry) error) error {
	l.callback = callback
	return nil
}

func (l *casbinLogger) loggerForEntry(entry *casbinlog.LogEntry) *slog.Logger {
	entryLogger := slog.With("component", "casbin", "event_type", string(entry.EventType))
	if entry.Subject != "" {
		entryLogger = entryLogger.With("subject", entry.Subject)
	}
	if entry.Object != "" {
		entryLogger = entryLogger.With("object", entry.Object)
	}
	if entry.Action != "" {
		entryLogger = entryLogger.With("action", entry.Action)
	}
	if entry.Domain != "" {
		entryLogger = entryLogger.With("domain", entry.Domain)
	}
	return entryLogger
}

func (l *casbinLogger) shouldLog(eventType casbinlog.EventType) bool {
	if len(l.eventTypes) == 0 {
		return true
	}
	_, ok := l.eventTypes[eventType]
	return ok
}
