//go:build nostatsview
// +build nostatsview

package server

import (
	"net"
)

// ViewManager
type ViewManager struct{}

// Start runs a http server and begin to collect metrics
func (vm *ViewManager) Start() error {
	return nil
}

// Stop shutdown the http server gracefully
func (vm *ViewManager) Stop() {
}

// NewViewManager creates a new ViewManager instance
func NewViewManager(ln net.Listener, s *Server) *ViewManager {
	return new(ViewManager)
}
