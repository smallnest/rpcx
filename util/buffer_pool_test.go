package util

import (
	"fmt"
	"testing"
)

func TestLimitedPool_findPool(t *testing.T) {
	pool := NewLimitedPool(512, 4096)

	type args struct {
		size int
	}
	tests := []struct {
		args int
		want int
	}{
		{200, 512},
		{512, 512},
		{1000, 1024},
		{2000, 2048},
		{2048, 2048},
		{4095, 4096},
		{4096, 4096},
		{4097, -1},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("bytes-%d", tt.args), func(t *testing.T) {
			got := pool.findPool(tt.args)
			if got == nil {
				if tt.want > 0 {
					fmt.Errorf("expect %d pool but got nil", tt.want)
				}
				return
			}

			if got.size != tt.want {
				fmt.Errorf("expect %d pool but got %d pool", tt.want, got.size)
			}
		})
	}
}

func TestLimitedPool_findPutPool(t *testing.T) {
	pool := NewLimitedPool(512, 4096)

	type args struct {
		size int
	}
	tests := []struct {
		args int
		want int
	}{
		{200, -1}, //too small so we discard it
		{512, 512},
		{1000, 512},
		{2000, 1024},
		{2048, 2048},
		{4095, 2048},
		{4096, 4096},
		{4097, -1}, // too big so we discard it
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("bytes-%d", tt.args), func(t *testing.T) {
			got := pool.findPutPool(tt.args)
			if got == nil {
				if tt.want > 0 {
					fmt.Errorf("expect %d pool but got nil", tt.want)
				}
				return
			}

			if got.size != tt.want {
				fmt.Errorf("expect %d pool but got %d pool", tt.want, got.size)
			}
		})
	}
}
