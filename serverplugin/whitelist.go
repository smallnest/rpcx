package serverplugin

import "net"

// WhitelistPlugin is a plugin that control only ip addresses in whitelist can access services.
type WhitelistPlugin struct {
	Whitelist     map[string]bool
	WhitelistMask []*net.IPNet // net.ParseCIDR("172.17.0.0/16") to get *net.IPNet
}

// HandleConnAccept check ip.
func (plugin *WhitelistPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	ip, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return conn, false
	}
	if plugin.Whitelist[ip] {
		return conn, true
	}

	remoteIP := net.ParseIP(ip)
	for _, mask := range plugin.WhitelistMask {
		if mask.Contains(remoteIP) {
			return conn, true
		}
	}

	return conn, false
}
