package betterrpc

//IPlugin represents a plugin.
type IPlugin interface {
	Name() string
	Description() string
}
