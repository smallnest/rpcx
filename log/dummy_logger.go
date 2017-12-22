package log

type dummyLogger struct{}

func (l *dummyLogger) Debug(v ...interface{}) {
}

func (l *dummyLogger) Debugf(format string, v ...interface{}) {
}

func (l *dummyLogger) Info(v ...interface{}) {
}

func (l *dummyLogger) Infof(format string, v ...interface{}) {
}

func (l *dummyLogger) Warn(v ...interface{}) {
}

func (l *dummyLogger) Warnf(format string, v ...interface{}) {
}

func (l *dummyLogger) Error(v ...interface{}) {
}

func (l *dummyLogger) Errorf(format string, v ...interface{}) {
}

func (l *dummyLogger) Fatal(v ...interface{}) {
}

func (l *dummyLogger) Fatalf(format string, v ...interface{}) {
}

func (l *dummyLogger) Panic(v ...interface{}) {
}

func (l *dummyLogger) Panicf(format string, v ...interface{}) {
}
