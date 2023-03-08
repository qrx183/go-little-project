package geeRpc

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	numCalls  uint64
}

func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

func (m *methodType) newArgV() reflect.Value {
	var argV reflect.Value

	if m.ArgType.Kind() == reflect.Ptr {
		argV = reflect.New(m.ArgType.Elem())
	} else {
		argV = reflect.New(m.ArgType).Elem()
	}
	return argV
}

func (m *methodType) newReplyV() reflect.Value {
	replyV := reflect.New(m.ReplyType.Elem())

	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyV.Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replyV.Set(reflect.MakeMap(m.ReplyType.Elem()))
	}

	return replyV
}

type service struct {
	name   string
	typ    reflect.Type
	rcvr   reflect.Value // 作为第0个参数使用
	method map[string]*methodType
}

func newService(rcvr interface{}) *service {
	s := new(service)

	s.rcvr = reflect.ValueOf(rcvr)
	s.typ = reflect.TypeOf(rcvr)
	s.name = reflect.Indirect(reflect.ValueOf(rcvr)).Type().Name()

	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	s.registerMethods()
	return s
}

func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)

	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(0), mType.In(1)
		if !isExportedOrBuiltInType(argType) || !isExportedOrBuiltInType(replyType) {
			continue
		}

		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
	}
}

func isExportedOrBuiltInType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

// call 通过反射调用结构体对应的方法
func (s *service) call(m *methodType, argV, replyV reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)

	f := m.method.Func

	// 根据反射去调用该方法
	returnValues := f.Call([]reflect.Value{s.rcvr, argV, replyV})

	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}
