package plugin

import "testing"

var logMsg string

func logMessage(l string) {
	logMsg = l
}

func TestLogRegisterPlugin_Register(t *testing.T) {
	plugin := &LogRegisterPlugin{Log: logMessage}
	plugin.Register("ABC", "aService")
	if logMsg != "Registered Service ABC with aService" {
		t.Errorf("LogRegisterPlugin doesn't work")
	}
}
