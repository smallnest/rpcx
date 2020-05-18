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
	Service        string `json:"service,omitempty"`
	Meta           string `json:"meta,omitempty"`
	ServiceAddress string `json:"service_address,omitempty"`
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
	domain string

	dying chan struct{}
	done  chan struct{}
}

// NewMDNSRegisterPlugin return a new MDNSRegisterPlugin.
// If domain is empty, use "local." in default.
func NewMDNSRegisterPlugin(serviceAddress string, port int, m metrics.Registry, updateInterval time.Duration, domain string) *MDNSRegisterPlugin {
	if domain == "" {
		domain = "local."
	}
	return &MDNSRegisterPlugin{
		ServiceAddress: serviceAddress,
		port:           port,
		Metrics:        m,
		UpdateInterval: updateInterval,
		domain:         domain,
		dying:          make(chan struct{}),
		done:           make(chan struct{}),
	}
}

// Start starts to connect etcd cluster
func (p *MDNSRegisterPlugin) Start() error {

	if p.server == nil && len(p.Services) != 0 {
		p.initMDNS()
	}

	if p.UpdateInterval > 0 {
		ticker := time.NewTicker(p.UpdateInterval)
		go func() {
			defer p.server.Shutdown()

			for {
				// refresh service TTL
				select {
				case <-p.dying:
					close(p.done)
					return
				case <-ticker.C:
					if p.server == nil && len(p.Services) == 0 {
						break
					}

					var data []byte
					if p.Metrics != nil {
						clientMeter := metrics.GetOrRegisterMeter("clientMeter", p.Metrics)
						data = []byte(strconv.FormatInt(clientMeter.Count()/60, 10))
					}

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
			}
		}()
	}

	return nil
}

// Stop unregister all services.
func (p *MDNSRegisterPlugin) Stop() error {
	p.server.Shutdown()

	close(p.dying)
	<-p.done

	return nil
}
func (p *MDNSRegisterPlugin) initMDNS() {
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

	server, err := zeroconf.Register(host, "_rpcxservices", p.domain, p.port, []string{s}, nil)
	if err != nil {
		panic(err)
	}
	p.server = server
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
	if strings.TrimSpace(name) == "" {
		err = errors.New("Register service `name` can't be empty")
		return
	}

	sm := &serviceMeta{
		Service:        name,
		Meta:           metadata,
		ServiceAddress: p.ServiceAddress,
	}

	p.Services = append(p.Services, sm)

	if p.server == nil {
		p.initMDNS()
		return
	}

	ss, _ := json.Marshal(p.Services)
	s := url.QueryEscape(string(ss))
	p.server.SetText([]string{s})
	return
}

func (p *MDNSRegisterPlugin) RegisterFunction(serviceName, fname string, fn interface{}, metadata string) error {
	return p.Register(serviceName, fn, metadata)
}

func (p *MDNSRegisterPlugin) Unregister(name string) (err error) {
	if strings.TrimSpace(name) == "" {
		err = errors.New("Register service `name` can't be empty")
		return
	}

	var services = make([]*serviceMeta, 0, len(p.Services)-1)
	for _, meta := range p.Services {
		if meta.Service != name {
			services = append(services, meta)
		}
	}
	p.Services = services

	ss, _ := json.Marshal(p.Services)
	s := url.QueryEscape(string(ss))
	p.server.SetText([]string{s})

	// if p.server != nil {
	// 	p.server.Shutdown()
	// 	return
	// }

	return
}
