package geeRpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"geeRpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

const MagicNumber = 0x3bef5c

// Option 在报头规定报文内容如何编解码
// 规定客户端通过JSON编码Option部分,服务端通过JSON解码得到CodecType,然后由CodecType再去决定如何解码Body和Header
type Option struct {
	MagicNumber    int // 用来唯一标识是rpc请求
	CodecType      codec.Type
	ConnectTimeOut time.Duration
	HandleTimeOut  time.Duration
}

var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.GobType,
	ConnectTimeOut: time.Second * 10,
}

type Server struct {
	serviceMap sync.Map
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

func (s *Server) Register(rcvr interface{}) error {
	serv := newService(rcvr)

	if _, dup := s.serviceMap.LoadOrStore(serv.name, serv); dup {
		return errors.New("rpc: service already defined: " + serv.name)
	}
	return nil
}

func (s *Server) FindService(serviceMethod string) (svc *service, mType *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")

	if dot < 0 {
		err = errors.New("rpc server: service/method request ill formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]

	svci, ok := s.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc serve: can't find this service: " + serviceName)
		return
	}
	svc = svci.(*service)

	mType = svc.method[methodName]
	if mType == nil {
		err = errors.New("rpc server: can't find this method: " + methodName)
	}
	return
}

func Register(rcvr interface{}) error { return DefaultServer.Register(rcvr) }

func (s *Server) Accept(lis net.Listener, timeout time.Duration) {

	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error: ", err)
			continue
		}
		// 这里会并发执行请求的处理逻辑,所以不会影响accept的结束
		s.ServeConn(conn, timeout)
		fmt.Println("sss")
	}
}

// Accept provide to user
func Accept(lis net.Listener, timeout time.Duration) {
	DefaultServer.Accept(lis, timeout)
}
func (s *Server) ServeConn(conn net.Conn, timeout time.Duration) {
	defer func() { _ = conn.Close() }()
	var option Option
	fmt.Println("aaaa")
	err := json.NewDecoder(conn).Decode(&option)
	fmt.Println("cc")
	if err != nil {
		log.Println("rpc server: options err: ", err)
		return
	}
	fmt.Println("bb")
	if option.MagicNumber != MagicNumber {
		log.Println("rpc server: invalid magic number: ", option.MagicNumber)
		return
	}

	f := codec.NewCodeFuncMap[option.CodecType]

	if f == nil {
		log.Println("rpc server: invalid code type: ", option.CodecType)
		return
	}

	s.ServeCodec(f(conn), timeout)
}

var invalidRequest = struct{}{}

func (s *Server) ServeCodec(cc codec.Codec, timeout time.Duration) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)

	for {
		// 首先获取client请求
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				break // 请求为空,不需要去发送响应
			}
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		// 多个请求可以并发处理,但是需要逐一发送
		go s.handleRequest(cc, req, wg, sending, timeout)
	}
	// 需要等待所有请求结束才能退出
	wg.Wait()
	_ = cc.Close()
}

type Request struct {
	h            *codec.Header
	argv, replyv reflect.Value
	mType        *methodType
	scv          *service
}

func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var header codec.Header

	if err := cc.ReadHeader(&header); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error: ", err)
			return nil, err
		}
	}
	return &header, nil
}

func (s *Server) readRequest(cc codec.Codec) (*Request, error) {
	h, err := s.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &Request{h: h}

	// TODO 此时还无法得知arg的类型,因此暂时按string处理
	req.scv, req.mType, err = s.FindService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mType.newArgV()
	req.replyv = req.mType.newReplyV()

	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	// 这里的req.argv一定要传Interface(),不然gob解码会报错
	if err = cc.ReadBody(argvi); err != nil {
		log.Println("rpc server: read body error: ", err)
		return nil, err
	}
	return req, nil
}

func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	// server send message
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error: ", err)
	}
}

func (s *Server) handleRequest(cc codec.Codec, req *Request, wg *sync.WaitGroup, sending *sync.Mutex, timeout time.Duration) {

	defer wg.Done()

	// 这里利用通道去判断请求处理是否超时,这里的通道应该设置为有缓冲通道
	// 因为无缓冲通道在接受者从通道中取出信息执行的时候发送者才能将消息放入同道中人,如果接收者不执行接收操作,通道会一直堵塞,导致子协程无法正常退出,从而造成内存泄漏
	called := make(chan struct{}, 1)
	sent := make(chan struct{}, 1)

	go func() {
		err := req.scv.call(req.mType, req.argv, req.replyv)
		// 在请求处理结束后填充通道
		// 设置成无缓冲通道,发生超时以后这里会阻塞
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			sent <- struct{}{}
			return
		}
	}()

	// 这里的req.replyv一定要传.Interface(),不然gob解码会报错
	s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
	sent <- struct{}{}

	if timeout == 0 {
		<-called
		<-sent
		return
	}
	// select类似switch语句,是专门针对通道设计的,每个case的条件都必须是一个通道操作
	select {
	case <-time.After(timeout):
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
		s.sendResponse(cc, req.h, invalidRequest, sending)

	case <-called:
		// 如果请求没有被处理,这里会处于阻塞状态
		<-sent
	}
}
