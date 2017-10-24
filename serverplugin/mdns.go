package serverplugin

import (
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
	metrics "github.com/rcrowley/go-metrics"
)

type serviceMeta struct {
	Service        string
	Meta           string
	ServiceAddress string
}

// MDNSRegisterPlugin implements mdns/dns-sd registry.
type MDNSRegisterPlugin struct {
	// service address, for example, tcp@127.0.0.1:8972, quic@127.0.0.1:1234
	ServiceAddress string
	port           int
	Metrics        metrics.Registry
	// Registered services
	Services       []*serviceMeta
	UpdateInterval time.Duration

	server *zeroconf.Server
}

// Start starts to connect etcd cluster
func (p *MDNSRegisterPlugin) Start() error {
	data, _ := json.Marshal(p.Services)
	s := url.QueryEscape(string(data))
	host, _ := os.Hostname()

	addr := p.ServiceAddress
	i := strings.Index(addr, "@")
	if i > 0 {
		addr = addr[i+1:]
	}
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	p.port, err = strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}

	server, err := zeroconf.Register(host, "_rpcxservices", "local.", p.port, []string{s}, nil)
	if err != nil {
		panic(err)
	}
	p.server = server

	if p.UpdateInterval > 0 {
		ticker := time.NewTicker(p.UpdateInterval)
		go func() {
			p.server.Shutdown()

			// refresh service TTL
			for range ticker.C {
				clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
				data := []byte(strconv.FormatInt(clientMeter.Count()/60, 10))
				//set this same metrics for all services at this server
				for _, sm := range p.Services {
					v, _ := url.ParseQuery(string(sm.Meta))
					v.Set("tps", string(data))
					sm.Meta = v.Encode()
				}
				ss, _ := json.Marshal(p.Services)
				s := url.QueryEscape(string(ss))
				p.server.SetText([]string{s})
			}
		}()
	}

	return nil
}

// HandleConnAccept handles connections from clients
func (p *MDNSRegisterPlugin) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	if p.Metrics != nil {
		clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
		clientMeter.Mark(1)
	}
	return conn, true
}

// Register handles registering event.
// this service is registered at BASE/serviceName/thisIpAddress node
func (p *MDNSRegisterPlugin) Register(name string, rcvr interface{}, metadata string) (err error) {
	if "" == strings.TrimSpace(name) {
		err = errors.New("Register service `name` can't be empty")
		return
	}
	if p.server == nil {
		return errors.New("MDNSRegisterPlugin has not started")
	}

	sm := &serviceMeta{
		Service:        name,
		Meta:           metadata,
		ServiceAddress: p.ServiceAddress,
	}

	p.Services = append(p.Services, sm)
	ss, _ := json.Marshal(p.Services)
	s := url.QueryEscape(string(ss))
	p.server.SetText([]string{s})
	return
}
