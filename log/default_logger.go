package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/fatih/color"
)

// The default logger output at level error, to turn everything on like this:
// 	export RPCX_LOG_LEVEL=99

// type of log level
type Level int

// log level constant
const (
	LvPanic Level = iota
	LvFatal
	LvError
	LvWarn
	LvInfo
	LvDebug

	LvMax
)

type outputFn func(calldepth int, s string) error

func dropOutput(calldepth int, s string) error {
	return nil
}

type defaultLogger struct {
	*log.Logger
	out [LvMax]outputFn
}

func NewDefaultLogger(out io.Writer, prefix string, flag int, lv Level) *defaultLogger {
	// set default level to error
	level := lv

	// read level from system enviroment to override
	levelStr := os.Getenv("RPCX_LOG_LEVEL")
	if len(levelStr) != 0 {
		if vl, err := strconv.Atoi(levelStr); err == nil {
			level = Level(vl)
		}
	}

	l := &defaultLogger{}
	l.Logger = log.New(out, prefix, flag)

	for i := int(LvFatal); i < int(LvMax); i++ {
		// always enable fatal and panic

		// enable only lv <= RPCX_LOG_LEVEL
		if i <= int(level) {
			l.out[i] = l.Output
		} else {
			l.out[i] = dropOutput
		}
	}
	return l
}

func (l *defaultLogger) Debug(v ...interface{}) {
	_ = l.out[int(LvDebug)](calldepth, header("DEBUG", fmt.Sprint(v...)))
}

func (l *defaultLogger) Debugf(format string, v ...interface{}) {
	_ = l.out[int(LvDebug)](calldepth, header("DEBUG", fmt.Sprintf(format, v...)))
}

func (l *defaultLogger) Info(v ...interface{}) {
	_ = l.out[int(LvInfo)](calldepth, header(color.GreenString("INFO "), fmt.Sprint(v...)))
}

func (l *defaultLogger) Infof(format string, v ...interface{}) {
	_ = l.out[int(LvInfo)](calldepth, header(color.GreenString("INFO "), fmt.Sprintf(format, v...)))
}

func (l *defaultLogger) Warn(v ...interface{}) {
	_ = l.out[int(LvWarn)](calldepth, header(color.YellowString("WARN "), fmt.Sprint(v...)))
}

func (l *defaultLogger) Warnf(format string, v ...interface{}) {
	_ = l.out[int(LvWarn)](calldepth, header(color.YellowString("WARN "), fmt.Sprintf(format, v...)))
}

func (l *defaultLogger) Error(v ...interface{}) {
	_ = l.out[int(LvError)](calldepth, header(color.RedString("ERROR"), fmt.Sprint(v...)))
}

func (l *defaultLogger) Errorf(format string, v ...interface{}) {
	_ = l.out[int(LvError)](calldepth, header(color.RedString("ERROR"), fmt.Sprintf(format, v...)))
}

func (l *defaultLogger) Fatal(v ...interface{}) {
	_ = l.Logger.Output(calldepth, header(color.MagentaString("FATAL"), fmt.Sprint(v...)))
	os.Exit(1)
}

func (l *defaultLogger) Fatalf(format string, v ...interface{}) {
	_ = l.Logger.Output(calldepth, header(color.MagentaString("FATAL"), fmt.Sprintf(format, v...)))
	os.Exit(1)
}

func (l *defaultLogger) Panic(v ...interface{}) {
	l.Logger.Panic(v...)
}

func (l *defaultLogger) Panicf(format string, v ...interface{}) {
	l.Logger.Panicf(format, v...)
}

func header(lvl, msg string) string {
	return fmt.Sprintf("%s: %s", lvl, msg)
}
