package codec

// import (
// 	"bufio"
// 	"errors"
// 	"io"
// 	"log"
// 	"net/rpc"
// 	"reflect"

// 	hessian "github.com/smallnest/gohessian"
// )

// // hessianServerCodec is exacted from go net/rpc/server.go
// type hessianServerCodec struct {
// 	rwc     io.ReadWriteCloser
// 	dec     *hessian.Decoder
// 	enc     *hessian.Encoder
// 	encBuf  *bufio.Writer
// 	typeMap map[string]reflect.Type
// 	nameMap map[string]string
// }

// // NewHessianServerCodec creates a gob ServerCodec
// func NewHessianServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
// 	buf := bufio.NewWriter(conn)
// 	cc := &hessianServerCodec{
// 		typeMap: make(map[string]reflect.Type, 17),
// 		nameMap: make(map[string]string, 17),
// 		rwc:     conn,
// 		encBuf:  buf,
// 	}

// 	cc.dec = hessian.NewDecoder(conn, cc.typeMap)
// 	cc.enc = hessian.NewEncoder(cc.encBuf, cc.nameMap)

// 	emptyRequest := rpc.Request{}
// 	typ := reflect.TypeOf(emptyRequest)
// 	cc.enc.RegisterNameType(typ.Name(), typ.Name())
// 	cc.dec.RegisterType(typ.Name(), typ)

// 	emptyResponse := rpc.Response{}
// 	typ = reflect.TypeOf(emptyResponse)
// 	cc.enc.RegisterNameType(typ.Name(), typ.Name())
// 	cc.dec.RegisterType(typ.Name(), typ)

// 	return cc
// }

// func (c *hessianServerCodec) ReadRequestHeader(r *rpc.Request) error {
// 	obj, err := c.dec.ReadObject()
// 	if err != nil {
// 		panic(err)
// 		return err
// 	}
// 	req, ok := obj.(rpc.Request)
// 	if !ok {
// 		return errors.New("failed to read *rpc.Request")
// 	}
// 	*r = req
// 	return nil
// }

// func (c *hessianServerCodec) ReadRequestBody(body interface{}) error {
// 	obj, err := c.dec.ReadObject()
// 	if err != nil {
// 		return err
// 	}

// 	val := reflect.ValueOf(body)
// 	if val.Kind() != reflect.Ptr {
// 		return errors.New("body must be a pointer")
// 	}
// 	val = val.Elem()

// 	objValue := reflect.ValueOf(obj)
// 	if objValue.Kind() == reflect.Ptr {
// 		val.Set(objValue.Elem())
// 	} else {
// 		val.Set(objValue)
// 	}

// 	return nil
// }

// func (c *hessianServerCodec) WriteResponse(r *rpc.Response, body interface{}) (err error) {
// 	if _, err = c.enc.WriteObject(r); err != nil {
// 		if c.encBuf.Flush() == nil {
// 			// Gob couldn't encode the header. Should not happen, so if it does,
// 			// shut down the connection to signal that the connection is broken.
// 			log.Infof("rpc: gob error encoding response:", err)
// 			c.Close()
// 		}
// 		return
// 	}
// 	if _, err = c.enc.WriteObject(body); err != nil {
// 		if c.encBuf.Flush() == nil {
// 			// Was a gob problem encoding the body but the header has been written.
// 			// Shut down the connection to signal that the connection is broken.
// 			log.Infof("rpc: gob error encoding body:", err)
// 			c.Close()
// 		}
// 		return
// 	}
// 	return c.encBuf.Flush()
// }

// func (c *hessianServerCodec) Close() error {
// 	return c.rwc.Close()
// }

// type hessianClientCodec struct {
// 	rwc     io.ReadWriteCloser
// 	dec     *hessian.Decoder
// 	enc     *hessian.Encoder
// 	encBuf  *bufio.Writer
// 	typeMap map[string]reflect.Type
// 	nameMap map[string]string
// }

// // NewHessianClientCodec creates a gob ClientCodec
// func NewHessianClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
// 	encBuf := bufio.NewWriter(conn)
// 	cc := &hessianClientCodec{
// 		typeMap: make(map[string]reflect.Type, 17),
// 		nameMap: make(map[string]string, 17),
// 		rwc:     conn,
// 		encBuf:  encBuf}

// 	cc.dec = hessian.NewDecoder(conn, cc.typeMap)
// 	cc.enc = hessian.NewEncoder(cc.encBuf, cc.nameMap)

// 	emptyRequest := rpc.Request{}
// 	typ := reflect.TypeOf(emptyRequest)
// 	cc.enc.RegisterNameType(typ.Name(), typ.Name())
// 	cc.dec.RegisterType(typ.Name(), typ)

// 	emptyResponse := rpc.Response{}
// 	typ = reflect.TypeOf(emptyResponse)
// 	cc.enc.RegisterNameType(typ.Name(), typ.Name())
// 	cc.dec.RegisterType(typ.Name(), typ)

// 	return cc
// }

// func (c *hessianClientCodec) WriteRequest(r *rpc.Request, body interface{}) (err error) {
// 	if _, err = c.enc.WriteObject(r); err != nil {
// 		return
// 	}

// 	if _, err = c.enc.WriteObject(body); err != nil {
// 		return
// 	}
// 	return c.encBuf.Flush()
// }

// func (c *hessianClientCodec) ReadResponseHeader(r *rpc.Response) error {
// 	obj, err := c.dec.ReadObject()
// 	if err != nil {
// 		return err
// 	}
// 	resp, ok := obj.(rpc.Response)
// 	if !ok {
// 		return errors.New("failed to read *rpc.Request")
// 	}
// 	*r = resp
// 	return nil
// }

// func (c *hessianClientCodec) ReadResponseBody(body interface{}) error {
// 	obj, err := c.dec.ReadObject()
// 	if err != nil {
// 		return err
// 	}

// 	val := reflect.ValueOf(body)
// 	if val.Kind() != reflect.Ptr {
// 		return errors.New("body must be a pointer")
// 	}
// 	val = val.Elem()

// 	objValue := reflect.ValueOf(obj)
// 	if objValue.Kind() == reflect.Ptr {
// 		val.Set(objValue.Elem())
// 	} else {
// 		val.Set(objValue)
// 	}

// 	return nil
// }

// func (c *hessianClientCodec) Close() error {
// 	return c.rwc.Close()
// }
