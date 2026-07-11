package casbin

import (
	"shadmin/pkg"

	casbinlog "github.com/casbin/casbin/v3/log"
	"github.com/sirupsen/logrus"
)

var logger = pkg.Log

func casLog(action, message string) {
	logger.WithField("component", "casbin").WithField("action", action).Info(message)
}

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

	l.loggerForEntry(entry).Debug("casbin event started")
	return nil
}

func (l *casbinLogger) OnAfterEvent(entry *casbinlog.LogEntry) error {
	logger := l.loggerForEntry(entry)
	if entry.Error != nil {
		logger.WithError(entry.Error).Error("casbin event finished")
	} else {
		logger.WithField("allowed", entry.Allowed).WithField("duration_ms", entry.Duration.Milliseconds()).Info("casbin event finished")
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
	logger := pkg.Log.WithField("component", "casbin").WithField("event_type", string(entry.EventType))
	if entry.Subject != "" {
		logger = logger.WithField("subject", entry.Subject)
	}
	if entry.Object != "" {
		logger = logger.WithField("object", entry.Object)
	}
	if entry.Action != "" {
		logger = logger.WithField("action", entry.Action)
	}
	if entry.Domain != "" {
		logger = logger.WithField("domain", entry.Domain)
	}
	return logger
}

func (l *casbinLogger) shouldLog(eventType casbinlog.EventType) bool {
	if len(l.eventTypes) == 0 {
		return true
	}
	_, ok := l.eventTypes[eventType]
	return ok
}
