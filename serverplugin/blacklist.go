package serverplugin

import "net"

// BlacklistPlugin is a plugin that control only ip addresses in blacklist can **NOT** access services.
type BlacklistPlugin struct {
	Blacklist     map[string]bool
	BlacklistMask []*net.IPNet // net.ParseCIDR("172.17.0.0/16") to get *net.IPNet
}

// HandleConnAccept check ip.
func (plugin *BlacklistPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	ip, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return conn, true
	}
	if plugin.Blacklist[ip] {
		return conn, false
	}

	remoteIP := net.ParseIP(ip)
	for _, mask := range plugin.BlacklistMask {
		if mask.Contains(remoteIP) {
			return conn, false
		}
	}

	return conn, true
}
