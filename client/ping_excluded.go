// +build !ping

package client

import (
	"context"
	"errors"
)

func newWeightedICMPSelector(servers map[string]string) Selector {
	panic(errors.New("this lib has not been with tag 'ping' "))
}

func (s weightedICMPSelector) Select(ctx context.Context, servicePath, serviceMethod string, args interface{}) string {
	return ""
}

func (s *weightedICMPSelector) UpdateServer(servers map[string]string) {

}
