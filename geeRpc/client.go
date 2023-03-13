package geeRpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geeRpc/codec"
	"geeRpc/xclient"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
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

type XClient struct {
	d       xclient.Discovery
	mode    xclient.SelectMode
	mu      sync.Mutex
	opt     *Option
	clients map[string]*Client
}

func NewXClient(d xclient.Discovery, mode xclient.SelectMode, opt *Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

func (x *XClient) Close() error {
	x.mu.Lock()
	defer x.mu.Unlock()

	for k, v := range x.clients {
		_ = v.Close()
		delete(x.clients, k)
	}
	return nil
}

func (x *XClient) dial(rpcAddr string) (*Client, error) {
	client, ok := x.clients[rpcAddr]

	if ok && !client.IsAvailable() {
		// handle the client is not useful
		_ = client.Close()
		delete(x.clients, rpcAddr)
		client = nil
	}

	if client == nil {
		client, err := XDial(rpcAddr, x.opt)
		if err != nil {
			return nil, err
		}
		x.clients[rpcAddr] = client
	}
	return client, nil
}

func (x *XClient) call(rpcAddr string, ctx context.Context, argS, replyV interface{}, serviceMethod string) error {
	client, err := x.dial(rpcAddr)
	if err != nil {
		return err
	}

	return client.Call(ctx, serviceMethod, argS, replyV)
}

func (x *XClient) Call(ctx context.Context, args, replyV interface{}, serviceMethod string) error {
	rpcAddr, err := x.d.Get(x.mode)

	if err != nil {
		return err
	}
	return x.call(rpcAddr, ctx, args, replyV, serviceMethod)
}

// BroadCast
// 向所有服务实例进行广播,有一个错误即返回错误,有一个成功返回即返回该结果
func (x *XClient) BroadCast(ctx context.Context, serviceMethod string, argS, reply interface{}) error {
	servers, err := x.d.GetAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex

	replyDone := reply == nil
	var e error
	ctx, cancel := context.WithCancel(ctx)
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var clonedReply interface{}

			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Type().Elem()).Interface()
			}

			err := x.call(rpcAddr, ctx, argS, clonedReply, serviceMethod)
			// 如果不加锁并发情况下 reply和err可能被不正常赋值
			// 通过加锁保证err和reply被正确赋值
			mu.Lock()
			if err != nil && e == nil {
				e = err
				// 如果发生错误直接终止协程   快速失败
				cancel()
			}

			if err == nil && !replyDone {
				// 使用反射进行赋值可以保证reply指针的指向不发生改变,从而让clonedReply可以被正常回收
				// reply = clonedReply
				reflect.ValueOf(reply).Set(reflect.ValueOf(clonedReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return e
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

func NewHTTPClient(conn net.Conn, opt *Option) (*Client, error) {
	_, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", defaultPCPath))

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connected {
		return NewClient(conn, opt)
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	return nil, err
}

func DialHTTP(network, addr string, opts ...*Option) (*Client, error) {
	return dialTimeOut(NewHTTPClient, network, addr, opts...)
}

func XDial(rpcAddr string, opts ...*Option) (*Client, error) {
	parts := strings.Split(rpcAddr, "@")

	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc client err: wrong format '%s', expect protocol@addr", rpcAddr)
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "HTTP":
		return DialHTTP("tcp", addr, opts...)
	default:
		return Dial(protocol, addr, opts...)
	}
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
