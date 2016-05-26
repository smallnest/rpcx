package clientselector

import (
	"errors"
	"math/rand"
	"net/rpc"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
	"github.com/smallnest/rpcx"
)

// EtcdClientSelector is used to select a rpc server from etcd.
type EtcdClientSelector struct {
	EtcdServers    []string
	KeysAPI        client.KeysAPI
	ticker         *time.Ticker
	sessionTimeout time.Duration
	BasePath       string //should endwith serviceName
	Servers        []string
	SelectMode     rpcx.SelectMode
	timeout        time.Duration
	rnd            *rand.Rand
	currentServer  int
	len            int
}

// NewEtcdClientSelector creates a EtcdClientSelector
func NewEtcdClientSelector(etcdServers []string, sessionTimeout time.Duration, sm rpcx.SelectMode, timeout time.Duration) *EtcdClientSelector {
	selector := &EtcdClientSelector{
		EtcdServers:    etcdServers,
		sessionTimeout: sessionTimeout,
		SelectMode:     sm,
		timeout:        timeout,
		rnd:            rand.New(rand.NewSource(time.Now().UnixNano()))}

	selector.start()
	return selector
}

func (s *EtcdClientSelector) start() {
	cli, err := client.New(client.Config{
		Endpoints:               s.EtcdServers,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: s.sessionTimeout,
	})

	if err != nil {
		return
	}
	s.KeysAPI = client.NewKeysAPI(cli)

	s.ticker = time.NewTicker(s.sessionTimeout)
	go func() {
		for _ = range s.ticker.C {
			resp, err := s.KeysAPI.Get(context.TODO(), s.BasePath, &client.GetOptions{
				Recursive: true,
				Sort:      true,
			})
			if err == nil && resp.Node != nil {
				if len(resp.Node.Nodes) > 0 {
					servers := make([]string, len(resp.Node.Nodes))
					for _, n := range resp.Node.Nodes {
						servers = append(servers, n.Value)
					}
					s.Servers = servers
				}
			}
		}
	}()
}

//Select returns a rpc client
func (s *EtcdClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc) (*rpc.Client, error) {
	if s.SelectMode == rpcx.RandomSelect {
		s.currentServer = s.rnd.Intn(s.len)
		server := s.Servers[s.currentServer]
		ss := strings.Split(server, "@") //tcp@ip , tcp4@ip or tcp6@ip
		return rpcx.NewDirectRPCClient(clientCodecFunc, ss[0], ss[1], s.timeout)

	} else if s.SelectMode == rpcx.RandomSelect {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		server := s.Servers[s.currentServer]
		ss := strings.Split(server, "@") //
		return rpcx.NewDirectRPCClient(clientCodecFunc, ss[0], ss[1], s.timeout)

	} else {
		return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())
	}
}
