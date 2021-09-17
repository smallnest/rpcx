package server

// ShouldCloseConnError is a fatal error which indicates rpcx should close the connnection.
type ShouldCloseConnError interface {
	ShouldCloseConn() bool
}

type ShouldCloseConn struct {
	err             error
	shouldCloseConn bool
}

func (e *ShouldCloseConn) Error() string {
	return e.Error()
}

func (e *ShouldCloseConn) ShouldCloseConn() bool {
	return e.shouldCloseConn
}

func WrapErrorAsShouldCloseConnError(err error, shouldCloseConn bool) error {
	return &ShouldCloseConn{
		err:             err,
		shouldCloseConn: shouldCloseConn,
	}
}
