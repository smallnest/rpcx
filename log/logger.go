package log

import (
	"log"
	"os"
)

const (
	calldepth = 3
)

var l Logger = NewDefaultLogger(os.Stdout, "", log.LstdFlags|log.Lshortfile, LvError)

type Logger interface {
	Debug(v ...any)
	Debugf(format string, v ...any)

	Info(v ...any)
	Infof(format string, v ...any)

	Warn(v ...any)
	Warnf(format string, v ...any)

	Error(v ...any)
	Errorf(format string, v ...any)

	Fatal(v ...any)
	Fatalf(format string, v ...any)

	Panic(v ...any)
	Panicf(format string, v ...any)
}

func SetLogger(logger Logger) {
	l = logger
}

func GetLogger() Logger {
	return l
}

func SetDummyLogger() {
	l = &dummyLogger{}
}

func Debug(v ...any) {
	l.Debug(v...)
}
func Debugf(format string, v ...any) {
	l.Debugf(format, v...)
}

func Info(v ...any) {
	l.Info(v...)
}
func Infof(format string, v ...any) {
	l.Infof(format, v...)
}

func Warn(v ...any) {
	l.Warn(v...)
}
func Warnf(format string, v ...any) {
	l.Warnf(format, v...)
}

func Error(v ...any) {
	l.Error(v...)
}
func Errorf(format string, v ...any) {
	l.Errorf(format, v...)
}

func Fatal(v ...any) {
	l.Fatal(v...)
}
func Fatalf(format string, v ...any) {
	l.Fatalf(format, v...)
}

func Panic(v ...any) {
	l.Panic(v...)
}
func Panicf(format string, v ...any) {
	l.Panicf(format, v...)
}
