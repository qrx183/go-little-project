package geeRpc

import (
	"encoding/json"
	"fmt"
	"geeRpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0x3bef5c

// Option 在报头规定报文内容如何编解码
// 规定客户端通过JSON编码Option部分,服务端通过JSON解码得到CodecType,然后由CodecType再去决定如何解码Body和Header
type Option struct {
	MagicNumber int // 用来唯一标识是rpc请求
	CodecType   codec.Type
}

var DefaultOption = &Option{MagicNumber: MagicNumber, CodecType: codec.GobType}

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

func (s *Server) Accept(lis net.Listener) {

	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error: ", err)
			continue
		}
		// 这里会并发执行请求的处理逻辑,所以不会影响accept的结束
		s.ServeConn(conn)
		fmt.Println("sss")
	}
}

// Accept provide to user
func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}
func (s *Server) ServeConn(conn net.Conn) {
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

	s.ServeCodec(f(conn))
}

var invalidRequest = struct{}{}

func (s *Server) ServeCodec(cc codec.Codec) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)

	for {
		// 首先获取client请求
		req, err := s.ReadRequest(cc)
		if err != nil {
			if req == nil {
				break // 请求为空,不需要去发送响应
			}
			req.h.Error = err.Error()
			s.SendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		// 多个请求可以并发处理,但是需要逐一发送
		go s.HandleRequest(cc, req, wg, sending)
	}
	// 需要等待所有请求结束才能退出
	wg.Wait()
	_ = cc.Close()
}

type Request struct {
	h            *codec.Header
	argv, replyv reflect.Value
}

func (s *Server) ReadRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var header codec.Header

	if err := cc.ReadHeader(&header); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error: ", err)
			return nil, err
		}
	}
	return &header, nil
}

func (s *Server) ReadRequest(cc codec.Codec) (*Request, error) {
	h, err := s.ReadRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &Request{h: h}

	// TODO 此时还无法得知arg的类型,因此暂时按string处理
	req.argv = reflect.New(reflect.TypeOf(""))
	// 这里的req.argv一定要传Interface(),不然gob解码会报错
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read body error: ", err)
		return nil, err
	}
	return req, nil
}

func (s *Server) SendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	// server send message
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error: ", err)
	}
}

func (s *Server) HandleRequest(cc codec.Codec, req *Request, wg *sync.WaitGroup, sending *sync.Mutex) {
	// TODO 暂时先按照输出请求参数来处理请求
	defer wg.Done()
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq))
	// 这里的req.replyv一定要传.Interface(),不然gob解码会报错
	s.SendResponse(cc, req.h, req.replyv.Interface(), sending)
}
