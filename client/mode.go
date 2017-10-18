package client

//FailMode decides how clients action when clients fail to invoke services
type FailMode int

const (
	//Failover selects another server automaticaly
	Failover FailMode = iota
	//Failfast returns error immediately
	Failfast
	//Failtry use current client again
	Failtry
	//Broadcast sends requests to all servers and Success only when all servers return OK
	Broadcast
	//Forking sends requests to all servers and Success once one server returns OK
	Forking
)

// SelectMode defines the algorithm of selecting a services from candidates.
type SelectMode int

const (
	//RandomSelect is selecting randomly
	RandomSelect SelectMode = iota
	//RoundRobin is selecting by round robin
	RoundRobin
	//WeightedRoundRobin is selecting by weighted round robin
	WeightedRoundRobin
	//WeightedICMP is selecting by weighted Ping time
	WeightedICMP
	//ConsistentHash is selecting by hashing
	ConsistentHash
	//Closest is selecting the closest server
	Closest
)

var selectModeStrs = [...]string{
	"RandomSelect",
	"RoundRobin",
	"WeightedRoundRobin",
	"WeightedICMP",
	"ConsistentHash",
	"Closest",
}

func (s SelectMode) String() string {
	return selectModeStrs[s]
}
