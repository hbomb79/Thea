package logger

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
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

func (e LogStatus) Color() *color.Color {
	return []*color.Color{
		color.New(color.FgWhite, color.Italic),                //Debug
		color.New(color.FgHiGreen),                            //Success
		color.New(color.FgWhite),                              //Info
		color.New(color.FgGreen, color.Italic),                //New
		color.New(color.FgYellow, color.Italic),               //Remove
		color.New(color.FgRed),                                //Stop
		color.New(color.FgYellow, color.Underline),            //Warning
		color.New(color.FgHiRed, color.Bold),                  //Error
		color.New(color.FgHiRed, color.Bold, color.Underline), //PANIC
	}[e]
}

type Logger interface {
	Emit(LogStatus, string, ...interface{})
}

type loggerImpl struct {
	name string
}

func (l *loggerImpl) Emit(status LogStatus, message string, interpolations ...interface{}) {
	Log.Emit(status, l.name, message, interpolations...)
}

type LoggerManager interface {
	GetLogger(string) Logger
	Emit(LogStatus, string, string, ...interface{})
}

var Log LoggerManager = &loggerMgr{
	offset: 0,
}

type loggerMgr struct {
	offset int
}

func (l *loggerMgr) GetLogger(name string) Logger {
	return &loggerImpl{name: name}
}

func (l *loggerMgr) Emit(status LogStatus, name string, message string, interpolations ...interface{}) {
	l.setNameOffset(len(name))
	padding := strings.Repeat(" ", l.offset-len(name))
	msg := fmt.Sprintf("[%s] %s(%s) %s", name, padding, status, fmt.Sprintf(message, interpolations...))

	status.Color().Print(msg)
}

func (l *loggerMgr) setNameOffset(offset int) {
	if offset > l.offset {
		l.offset = offset
	}
}

func Get(name string) Logger {
	return Log.GetLogger(name)
}
