package casbin

import (
	"shadmin/pkg"

	casbinlog "github.com/casbin/casbin/v3/log"
	"github.com/sirupsen/logrus"
)

var logger = pkg.Log

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

func (l *casbinLogger) OnBeforeEvent(entry *casbinlog.LogEntry) error {
	if !l.shouldLog(entry.EventType) {
		return nil
	}
	return nil
}

func (l *casbinLogger) OnAfterEvent(entry *casbinlog.LogEntry) error {
	if !l.shouldLog(entry.EventType) {
		return nil
	}

	logger := l.loggerForEntry(entry)
	switch {
	case entry.Error != nil:
		logger.WithError(entry.Error).Error("casbin event failed")
	case entry.EventType == casbinlog.EventEnforce && !entry.Allowed:
		logger.WithField("allowed", false).Warn("casbin authorization denied")
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

func (l *casbinLogger) loggerForEntry(entry *casbinlog.LogEntry) *logrus.Entry {
	entryLogger := logger.WithField("component", "casbin").WithField("event_type", string(entry.EventType))
	if entry.Subject != "" {
		entryLogger = entryLogger.WithField("subject", entry.Subject)
	}
	if entry.Object != "" {
		entryLogger = entryLogger.WithField("object", entry.Object)
	}
	if entry.Action != "" {
		entryLogger = entryLogger.WithField("action", entry.Action)
	}
	if entry.Domain != "" {
		entryLogger = entryLogger.WithField("domain", entry.Domain)
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
