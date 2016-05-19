package clientselector

import (
	"errors"
	"math/rand"
	"net/rpc"
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
	"github.com/smallnest/betterrpc"
)

// ZooKeeperClientSelector is used to select a rpc server from zookeeper.
type ZooKeeperClientSelector struct {
	ZKServers      []string
	zkConn         *zk.Conn
	sessionTimeout time.Duration
	BasePath       string //should endwith serviceName
	Servers        []string
	SelectMode     betterrpc.SelectMode
	timeout        time.Duration
	rnd            *rand.Rand
	currentServer  int
	len            int
}

// NewZooKeeperClientSelector creates a ZooKeeperClientSelector
func NewZooKeeperClientSelector(zkServers []string, sessionTimeout time.Duration, sm betterrpc.SelectMode, timeout time.Duration) *ZooKeeperClientSelector {
	selector := &ZooKeeperClientSelector{
		ZKServers:      zkServers,
		sessionTimeout: sessionTimeout,
		SelectMode:     sm,
		timeout:        timeout,
		rnd:            rand.New(rand.NewSource(time.Now().UnixNano()))}

	selector.start()
	return selector
}

func (s *ZooKeeperClientSelector) start() {
	c, _, err := zk.Connect(s.ZKServers, s.sessionTimeout)
	if err != nil {
		panic(err)
	}

	s.zkConn = c
	exist, _, err := c.Exists(s.BasePath)
	if !exist {
		mkdirs(c, s.BasePath)
	}

	go s.watchPath()
}

func (s *ZooKeeperClientSelector) watchPath() {
	servers, _, ch, _ := s.zkConn.ChildrenW(s.BasePath)
	s.Servers = servers
	s.len = len(servers)
	// e := <-ch
	// if e.Type == zk.EventNodeChildrenChanged {

	// }
	<-ch
	s.watchPath()
}

//Select returns a rpc client
func (s *ZooKeeperClientSelector) Select(clientCodecFunc betterrpc.ClientCodecFunc) (*rpc.Client, error) {
	if s.SelectMode == betterrpc.RandomSelect {
		server := s.Servers[s.rnd.Intn(s.len)]
		ss := strings.Split(server, "@") //tcp@ip , tcp4@ip or tcp6@ip
		return betterrpc.NewDirectRPCClient(clientCodecFunc, ss[0], ss[1], s.timeout)

	} else if s.SelectMode == betterrpc.RandomSelect {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		server := s.Servers[s.currentServer]
		ss := strings.Split(server, "@") //
		return betterrpc.NewDirectRPCClient(clientCodecFunc, ss[0], ss[1], s.timeout)

	} else {
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
		exist, _, err = conn.Exists(createdPath)
		if !exist {
			path, err = conn.Create(createdPath, []byte(""), flags, acl)
			if err != nil {
				return
			}
		}
	}

	return nil
}
