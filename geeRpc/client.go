package geeRpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geeRpc/codec"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Call 远程调用
type Call struct {
	Seq           uint64
	ServiceMethod string
	Error         error
	Args          interface{}
	Reply         interface{}
	Done          chan *Call // ?支持异步调用?
}

func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	seq      uint64
	header   codec.Header
	cc       codec.Codec
	opt      *Option
	mu       sync.Mutex
	sending  sync.Mutex
	pending  map[uint64]*Call
	closing  bool // user has closed
	shutdown bool // server has told us stop
}

var ErrorShutDown = errors.New("connection is shut down")

var _ io.Closer = (*Client)(nil)

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrorShutDown
	}
	client.closing = true
	return nil
}

func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !(client.shutdown || client.closing)
}

// registerCall client send message invoke this method
func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.shutdown || client.closing {
		return 0, ErrorShutDown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++

	return call.Seq, nil
}

// removeCall client receive message invoke this method
func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.shutdown || client.closing {
		return nil
	}

	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()

	client.shutdown = true

	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

func (client *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err := client.cc.ReadHeader(&h); err != nil {
			break
		}
		call := client.removeCall(h.Seq)

		switch {
		case call == nil:
			// client send message happened some mistakes
			err = client.cc.ReadBody(nil)
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("read body " + err.Error())
			}
			call.done()
		}
	}
	// 非正常接收消息终止连接
	client.terminateCalls(err)
}

type clientResult struct {
	client *Client
	err    error
}

func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	f := codec.NewCodeFuncMap[opt.CodecType]

	if f == nil {
		err := fmt.Errorf("invalid codec type: %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}

	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: option encode error: ", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(cc codec.Codec, opt *Option) *Client {

	client := &Client{
		seq:     1,
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	// 并发调用receive接口,receive是一个无限循环的方法
	go client.receive()
	return client
}

func parseOption(opt ...*Option) (*Option, error) {

	if len(opt) == 0 || opt[0] == nil {
		return DefaultOption, nil
	}

	if len(opt) > 1 {
		return nil, errors.New("numbers of options is more than 1")
	}
	option := opt[0]
	option.MagicNumber = MagicNumber
	if option.CodecType == "" {
		option.CodecType = codec.GobType
	}
	return option, nil
}

func dialTimeOut(f newClientFunc, network, addr string, opts ...*Option) (client *Client, err error) {
	opt, err := parseOption(opts...)
	if err != nil {
		return nil, err
	}
	// 在建立连接时设置超时
	conn, err := net.DialTimeout(network, addr, opt.ConnectTimeOut)
	if err != nil {
		return nil, err
	}
	defer func() {
		// close the client if client is nil
		if client == nil {
			_ = conn.Close()
		}

	}()
	ch := make(chan clientResult, 1)
	go func() {
		client, err := f(conn, opt)
		ch <- clientResult{
			client: client,
			err:    err,
		}
	}()

	if opt.ConnectTimeOut == 0 {
		result := <-ch
		return result.client, result.err
	}

	// 接受超时做处理
	select {
	case <-time.After(opt.ConnectTimeOut):
		return nil, fmt.Errorf("rpc client: connect timeout: expect within %s", opt.ConnectTimeOut)
	case result := <-ch:
		return result.client, result.err
	}
}

type newClientFunc func(conn net.Conn, option *Option) (*Client, error)

func Dial(network, addr string, opts ...*Option) (client *Client, err error) {
	return dialTimeOut(NewClient, network, addr, opts...)
}

func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

	seq, err := client.registerCall(call)

	if err != nil {
		call.Error = err
		call.done()
		return
	}
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	if err = client.cc.Write(&client.header, call.Args); err != nil {
		call = client.removeCall(seq)

		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {

	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}

	call := &Call{
		ServiceMethod: serviceMethod,
		Reply:         reply,
		Args:          args,
		Done:          done,
	}
	// 这里其实可以用并发,因为call不需要client.send执行完再返回
	client.send(call)
	return call
}

func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	// use chan to choke
	// 这里是一个同步调用,拿不到结果就在信道中进行阻塞
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))

	/*
		也可以进行异步调用,在等待结果的时候可以考虑异步处理别的逻辑
		call := client.Go( ... )
		# 新启动协程，异步等待
		go func(call *Call) {
			select {
				<-call.Done:
					# do something
				<-otherChan:
					# do something
			}
		}(call)
	*/
	// 通过ctx将call的超时控制权交给用户
	select {
	case <-ctx.Done():
		client.removeCall(call.Seq)
		return errors.New("rpc client: call failed: " + ctx.Err().Error())
	case call := <-call.Done:
		return call.Error
	}
}
