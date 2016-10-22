package clientselector

import (
	"errors"
	"math/rand"
	"net"
	"net/rpc"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
	"github.com/smallnest/rpcx"
)

// ZooKeeperClientSelector is used to select a rpc server from zookeeper.
type ZooKeeperClientSelector struct {
	ZKServers          []string
	zkConn             *zk.Conn
	sessionTimeout     time.Duration
	BasePath           string //should endwith serviceName
	Servers            []string
	Group              string
	clientAndServer    map[string]*rpc.Client
	metadata           map[string]string
	Latitude           float64
	Longitude          float64
	WeightedServers    []*Weighted
	SelectMode         rpcx.SelectMode
	dailTimeout        time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *rpcx.Client
}

// NewZooKeeperClientSelector creates a ZooKeeperClientSelector
// sessionTimeout is timeout configuration for zookeeper.
// timeout is timeout configuration for TCP connection to RPC servers.
func NewZooKeeperClientSelector(zkServers []string, basePath string, sessionTimeout time.Duration, sm rpcx.SelectMode, dailTimeout time.Duration) *ZooKeeperClientSelector {
	selector := &ZooKeeperClientSelector{
		ZKServers:       zkServers,
		BasePath:        basePath,
		sessionTimeout:  sessionTimeout,
		SelectMode:      sm,
		clientAndServer: make(map[string]*rpc.Client),
		metadata:        make(map[string]string),
		dailTimeout:     dailTimeout,
		rnd:             rand.New(rand.NewSource(time.Now().UnixNano()))}

	selector.start()
	return selector
}

//SetClient sets a Client in order that clientSelector can uses it
func (s *ZooKeeperClientSelector) SetClient(c *rpcx.Client) {
	s.Client = c
}

//SetSelectMode sets SelectMode
func (s *ZooKeeperClientSelector) SetSelectMode(sm rpcx.SelectMode) {
	s.SelectMode = sm
}

//AllClients returns rpc.Clients to all servers
func (s *ZooKeeperClientSelector) AllClients(clientCodecFunc rpcx.ClientCodecFunc) []*rpc.Client {
	var clients []*rpc.Client

	for _, sv := range s.Servers {
		c, err := s.getCachedClient(sv, clientCodecFunc)
		if err == nil {
			clients = append(clients, c)
		}
	}

	return clients
}

func (s *ZooKeeperClientSelector) start() {
	c, _, err := zk.Connect(s.ZKServers, s.sessionTimeout)
	if err != nil {
		panic(err)
	}

	s.zkConn = c
	exist, _, _ := c.Exists(s.BasePath)
	if !exist {
		mkdirs(c, s.BasePath)
	}

	servers, _, _ := s.zkConn.Children(s.BasePath)
	s.Servers = servers

	s.createWeighted()

	//set weight based on ICMP result
	if s.SelectMode == rpcx.WeightedICMP {
		for _, w := range s.WeightedServers {
			server := w.Server.(string)
			ss := strings.Split(server, "@")
			host, _, _ := net.SplitHostPort(ss[1])
			rtt, _ := Ping(host)
			rtt = CalculateWeight(rtt)
			w.Weight = rtt
			w.EffectiveWeight = rtt
		}
	}

	s.len = len(s.Servers)
	if s.len > 0 {
		s.currentServer = s.currentServer % s.len
	}

	go s.watchPath()
}

func (s *ZooKeeperClientSelector) createWeighted() {
	s.WeightedServers = make([]*Weighted, len(s.Servers))

	var inactiveServers []int

	for i, ss := range s.Servers {
		bytes, _, err := s.zkConn.Get(s.BasePath + "/" + ss)
		s.WeightedServers[i] = &Weighted{Server: ss, Weight: 1, EffectiveWeight: 1}
		if err == nil {
			metadata := string(bytes)
			s.metadata[ss] = metadata
			if v, err := url.ParseQuery(metadata); err == nil {
				w := v.Get("weight")
				state := v.Get("state")
				group := v.Get("group")
				if (state != "" && state != "active") || s.Group != group {
					inactiveServers = append(inactiveServers, i)
				}

				if w != "" {
					weight, err := strconv.Atoi(w)
					if err != nil {
						s.WeightedServers[i].Weight = weight
						s.WeightedServers[i].EffectiveWeight = weight
					}
				}
			}
		}

	}

	s.removeInactiveServers(inactiveServers)
}

func (s *ZooKeeperClientSelector) watchPath() {
	servers, _, ch, _ := s.zkConn.ChildrenW(s.BasePath)
	s.Servers = servers
	s.len = len(servers)
	if s.SelectMode == rpcx.WeightedRoundRobin {
		s.createWeighted()
	}
	s.currentServer = s.currentServer % s.len
	// e := <-ch
	// if e.Type == zk.EventNodeChildrenChanged {

	// }
	<-ch
	s.watchPath()
}

func (s *ZooKeeperClientSelector) removeInactiveServers(inactiveServers []int) {
	i := len(inactiveServers) - 1
	for ; i >= 0; i-- {
		k := inactiveServers[i]
		removedServer := s.Servers[k]
		s.Servers = append(s.Servers[0:k], s.Servers[k+1:]...)
		s.WeightedServers = append(s.WeightedServers[0:k], s.WeightedServers[k+1:]...)

		c := s.clientAndServer[removedServer]
		if c != nil {
			delete(s.clientAndServer, removedServer)
			delete(s.metadata, removedServer)
			c.Close() //close connection to inactive server
		}
	}
}

func (s *ZooKeeperClientSelector) getCachedClient(server string, clientCodecFunc rpcx.ClientCodecFunc) (*rpc.Client, error) {
	c := s.clientAndServer[server]
	if c != nil {
		return c, nil
	}
	ss := strings.Split(server, "@") //
	c, err := rpcx.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)
	s.clientAndServer[server] = c
	return c, err
}

//Select returns a rpc client
func (s *ZooKeeperClientSelector) Select(clientCodecFunc rpcx.ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	if s.len == 0 {
		return nil, errors.New("No available service")
	}

	switch s.SelectMode {
	case rpcx.RandomSelect:
		s.currentServer = s.rnd.Intn(s.len)
		server := s.Servers[s.currentServer]
		return s.getCachedClient(server, clientCodecFunc)

	case rpcx.RoundRobin:
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		server := s.Servers[s.currentServer]
		return s.getCachedClient(server, clientCodecFunc)

	case rpcx.ConsistentHash:
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options)
		server := s.Servers[s.currentServer]
		return s.getCachedClient(server, clientCodecFunc)

	case rpcx.WeightedRoundRobin, rpcx.WeightedICMP:
		server := nextWeighted(s.WeightedServers).Server.(string)
		return s.getCachedClient(server, clientCodecFunc)

	case rpcx.Closest:
		closestServers := getClosestServer(s.Latitude, s.Longitude, s.metadata)
		selected := s.rnd.Intn(len(closestServers))
		return s.getCachedClient(closestServers[selected], clientCodecFunc)

	default:
		return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())

	}
}

func mkdirs(conn *zk.Conn, path string) (err error) {
	if path == "" {
		return errors.New("path should not been empty")
	}
	if path == "/" {
		return nil
	}
	if path[0] != '/' {
		return errors.New("path must start with /")
	}

	//check whether this path exists
	exist, _, err := conn.Exists(path)
	if exist {
		return nil
	}
	flags := int32(0)
	acl := zk.WorldACL(zk.PermAll)
	_, err = conn.Create(path, []byte(""), flags, acl)
	if err == nil { //created successfully
		return
	}

	//create parent
	paths := strings.Split(path[1:], "/")
	createdPath := ""
	for _, p := range paths {
		createdPath = createdPath + "/" + p
		exist, _, _ = conn.Exists(createdPath)
		if !exist {
			_, err = conn.Create(createdPath, []byte(""), flags, acl)
			if err != nil {
				return
			}
		}
	}

	return nil
}
