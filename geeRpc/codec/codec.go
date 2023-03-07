package codec

import "io"

type Header struct {
	ServiceMethod string // 请求方法
	Seq           uint64 // 序列ID,用来区分不同的请求
	Error         string // 响应的错误信息
}

// Codec
// 抽象出的对消息进行编解码的接口
type Codec interface {
	io.Closer
	ReadHeader(h *Header) error
	ReadBody(b interface{}) error
	Write(h *Header, b interface{}) error
}

type NewCodeFunc func(closer io.ReadWriteCloser) Codec

type Type string

const (
	GobType  = "application/gob"
	JsonType = "application/json"
)

var NewCodeFuncMap = map[Type]NewCodeFunc{}

func init() {
	NewCodeFuncMap = make(map[Type]NewCodeFunc)
	NewCodeFuncMap[GobType] = NewGobCodec
}
