package pkg

import (
	"fmt"
	"strings"
)

type LogContext int

const (
	ALL LogContext = iota
	CORE
	SERVICE_DB
	SERVICE_PGADMIN
	SERVICE_SITE
)

type LogStatus int

const (
	DEBUG LogStatus = iota
	SUCCESS
	INFO
	NEW
	REMOVE
	STOP
	WARNING
	ERROR
	FATAL
)

func (e LogStatus) String() string {
	return []string{
		"D",
		"✓",
		"I",
		"+",
		"-",
		"X",
		"!",
		"!!",
		"PANIC",
	}[e]
}

type LogListener func(LogStatus, string, LogContext)

type Logger interface {
	Emit(LogStatus, string, ...interface{})
}

type loggerImpl struct {
	name    string
	context LogContext
}

func (l *loggerImpl) Emit(status LogStatus, message string, interpolations ...interface{}) {
	Log.Emit(l.context, status, l.name, message, interpolations...)
}

type LoggerManager interface {
	AttachListener(LogListener, LogStatus, ...LogContext)
	GetLogger(string, LogContext) Logger
	Emit(LogContext, LogStatus, string, string, ...interface{})
}

var Log LoggerManager = &loggerMgr{
	offset: 0,
}

type loggerMgr struct {
	offset    int
	listeners map[LogContext][]struct {
		status   LogStatus
		listener LogListener
	}
}

func (l *loggerMgr) GetLogger(name string, context LogContext) Logger {
	return &loggerImpl{name: name}
}

func (l *loggerMgr) AttachListener(listener LogListener, minimumStatus LogStatus, contexts ...LogContext) {
	for _, ctx := range contexts {
		l.listeners[ctx] = append(l.listeners[ctx], struct {
			status   LogStatus
			listener LogListener
		}{status: minimumStatus, listener: listener})
	}
}

func (l *loggerMgr) Emit(ctx LogContext, status LogStatus, name string, message string, interpolations ...interface{}) {
	l.setNameOffset(len(name))
	padding := strings.Repeat(" ", l.offset-len(name))
	msg := fmt.Sprintf("[%s] %s(%s) %s", name, padding, status, fmt.Sprintf(message, interpolations...))

	listeners, ok := l.listeners[ctx]
	if !ok {
		fmt.Print(msg)
		return
	}

	for _, lst := range listeners {
		if lst.status <= status {
			lst.listener(status, msg, ctx)
		}
	}
}

func (l *loggerMgr) setNameOffset(offset int) {
	if offset > l.offset {
		l.offset = offset
	}
}
