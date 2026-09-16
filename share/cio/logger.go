package cio

import (
	"fmt"
	"io"
	"log"
	"os"
)

// Logger is pkg/log Logger with prefixing and 2 log levels
type Logger struct {
	Info, Debug bool
	//internal
	prefix      string
	logger      *log.Logger
	info, debug *bool
	hook        *func(string)
}

func NewLogger(prefix string) *Logger {
	return NewLoggerFlag(prefix, log.Ldate|log.Ltime)
}

func NewLoggerFlag(prefix string, flag int) *Logger {
	l := &Logger{
		prefix: prefix,
		logger: log.New(os.Stderr, "", flag),
		Info:   false,
		Debug:  false,
	}
	return l
}

func (l *Logger) Infof(f string, args ...interface{}) {
	msg := fmt.Sprintf(f, args...)
	if l.IsInfo() {
		l.logger.Printf("%s: %s", l.prefix, msg)
	}
	if l.hook != nil && *l.hook != nil {
		(*l.hook)(l.prefix + ": " + msg)
	}
}

func (l *Logger) Debugf(f string, args ...interface{}) {
	msg := fmt.Sprintf(f, args...)
	if l.IsDebug() {
		l.logger.Printf("%s: %s", l.prefix, msg)
	}
	if l.hook != nil && *l.hook != nil {
		(*l.hook)(l.prefix + ": " + msg)
	}
}

func (l *Logger) Errorf(f string, args ...interface{}) error {
	return fmt.Errorf(l.prefix+": "+f, args...)
}

// SetOutput sets the destination for logger output.
func (l *Logger) SetOutput(w io.Writer) {
	l.logger.SetOutput(w)
}

// SetHook attaches an observer callback to every logged message
func (l *Logger) SetHook(h func(string)) {
	if l.hook == nil {
		l.hook = &h
	} else {
		*l.hook = h
	}
}

func (l *Logger) Fork(prefix string, args ...interface{}) *Logger {
	//slip the parent prefix at the front
	args = append([]interface{}{l.prefix}, args...)
	ll := NewLogger(fmt.Sprintf("%s: "+prefix, args...))
	if l.hook == nil {
		var noop func(string)
		l.hook = &noop
	}
	ll.hook = l.hook
	//store link to parent settings too
	ll.Info = l.Info
	if l.info != nil {
		ll.info = l.info
	} else {
		ll.info = &l.Info
	}
	ll.Debug = l.Debug
	if l.debug != nil {
		ll.debug = l.debug
	} else {
		ll.debug = &l.Debug
	}
	return ll
}

func (l *Logger) Prefix() string {
	return l.prefix
}

func (l *Logger) IsInfo() bool {
	return l.Info || (l.info != nil && *l.info)
}

func (l *Logger) IsDebug() bool {
	return l.Debug || (l.debug != nil && *l.debug)
}
