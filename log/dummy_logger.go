package log

type dummyLogger struct{}

func (l *dummyLogger) Debug(v ...any) {
}

func (l *dummyLogger) Debugf(format string, v ...any) {
}

func (l *dummyLogger) Info(v ...any) {
}

func (l *dummyLogger) Infof(format string, v ...any) {
}

func (l *dummyLogger) Warn(v ...any) {
}

func (l *dummyLogger) Warnf(format string, v ...any) {
}

func (l *dummyLogger) Error(v ...any) {
}

func (l *dummyLogger) Errorf(format string, v ...any) {
}

func (l *dummyLogger) Fatal(v ...any) {
}

func (l *dummyLogger) Fatalf(format string, v ...any) {
}

func (l *dummyLogger) Panic(v ...any) {
}

func (l *dummyLogger) Panicf(format string, v ...any) {
}
