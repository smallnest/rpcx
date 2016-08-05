package plugin

//UDPRegisterPlugin is a register plugin which can register services by sending a UDP message.
//This registry is experimental and has not been test.
type UDPRegisterPlugin struct {
	UDPAddress string
	Services   []string
}

// Start starts to connect etcd cluster
func (plugin *UDPRegisterPlugin) Start() (err error) {

	return
}

//Close closes this plugin
func (plugin *UDPRegisterPlugin) Close() {

}

// Register handles registering event.
func (plugin *UDPRegisterPlugin) Register(name string, rcvr interface{}) (err error) {

	return
}

// Unregister a service from consul but this service still exists in this node.
func (plugin *UDPRegisterPlugin) Unregister(name string) {

}

// Name return name of this plugin.
func (plugin *UDPRegisterPlugin) Name() string {
	return "UDPRegisterPlugin"
}

// Description return description of this plugin.
func (plugin *UDPRegisterPlugin) Description() string {
	return "a register plugin which can register services into etcd for cluster"
}
