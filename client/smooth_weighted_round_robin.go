package client

// Weighted is a wrapped server with  weight
type Weighted struct {
	Server          string
	Weight          int
	CurrentWeight   int
	EffectiveWeight int
}
