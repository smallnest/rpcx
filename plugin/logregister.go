package plugin

import "fmt"

//LogRegisterPlugin is a register plugin which can log registered services in logs
type LogRegisterPlugin struct {
	Log func(log string)
}

// Register handles registering event.
func (plugin *LogRegisterPlugin) Register(name string, rcvr interface{}, metadata ...string) error {
	plugin.Log(fmt.Sprintf("Registered Service %s with %v", name, rcvr))
	return nil
}

// Name return name of this plugin.
func (plugin *LogRegisterPlugin) Name() string {
	return "LogRegisterPlugin"
}
