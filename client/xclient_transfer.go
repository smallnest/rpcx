package client

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"

	"github.com/juju/ratelimit"
	"github.com/smallnest/rpcx/share"
)

// File transfer (SendFile, DownloadFile) and bidirectional streaming (Stream)
// for xClient. Extracted from xclient.go.

// SendFile sends a local file to the server.
// fileName is the path of local file.
// rateInBytesPerSecond can limit bandwidth of sending,  0 means does not limit the bandwidth, unit is bytes / second.
func (c *xClient) SendFile(ctx context.Context, fileName string, rateInBytesPerSecond int64, meta map[string]string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}

	defer file.Close()

	fi, err := os.Stat(fileName)
	if err != nil {
		return err
	}

	args := share.FileTransferArgs{
		FileName: fi.Name(),
		FileSize: fi.Size(),
		Meta:     meta,
	}

	ctx = setServerTimeout(ctx)

	reply := &share.FileTransferReply{}
	err = c.Call(ctx, "TransferFile", args, reply)
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", reply.Addr, c.option.ConnectTimeout)
	if err != nil {
		return err
	}

	defer conn.Close()

	_, err = conn.Write(reply.Token)
	if err != nil {
		return err
	}

	var tb *ratelimit.Bucket

	if rateInBytesPerSecond > 0 {
		tb = ratelimit.NewBucketWithRate(float64(rateInBytesPerSecond), rateInBytesPerSecond)
	}

	sendBuffer := make([]byte, FileTransferBufferSize)
loop:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break loop
		default:
			if tb != nil {
				tb.Wait(FileTransferBufferSize)
			}
			n, err := file.Read(sendBuffer)
			if err != nil {
				if err == io.EOF {
					return nil
				} else {
					return err
				}
			}
			if n == 0 {
				break loop
			}
			_, err = conn.Write(sendBuffer[:n])
			if err != nil {
				if err == io.EOF {
					return nil
				} else {
					return err
				}
			}
		}
	}

	return nil
}

func (c *xClient) DownloadFile(ctx context.Context, requestFileName string, saveTo io.Writer, meta map[string]string) error {
	ctx = setServerTimeout(ctx)

	args := share.DownloadFileArgs{
		FileName: requestFileName,
		Meta:     meta,
	}

	reply := &share.FileTransferReply{}
	err := c.Call(ctx, "DownloadFile", args, reply)
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", reply.Addr, c.option.ConnectTimeout)
	if err != nil {
		return err
	}

	defer conn.Close()

	_, err = conn.Write(reply.Token)
	if err != nil {
		return err
	}

	buf := make([]byte, FileTransferBufferSize)
	r := bufio.NewReader(conn)
loop:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break loop
		default:
			n, er := r.Read(buf)
			if n > 0 {
				_, ew := saveTo.Write(buf[0:n])
				if ew != nil {
					err = ew
					break loop
				}
			}
			if er != nil {
				if er != io.EOF {
					err = er
				}
				break loop
			}
		}
	}

	return err
}

func (c *xClient) Stream(ctx context.Context, meta map[string]string) (net.Conn, error) {
	args := share.StreamServiceArgs{
		Meta: meta,
	}

	ctx = setServerTimeout(ctx)

	reply := &share.StreamServiceReply{}
	err := c.Call(ctx, "Stream", args, reply)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("tcp", reply.Addr, c.option.ConnectTimeout)
	if err != nil {
		return nil, err
	}

	_, err = conn.Write(reply.Token)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
