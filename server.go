package GeeRPC

import (
	"GeeRPC/codec"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0x3bef5c

// Option Option固定在报文开始 后面可接多个header和body
// option 使用JSON解码 之后使用CodecType解码剩余的头和消息体
type Option struct {
	// 标记这是一个RPC请求
	MagicNumber int
	// 用户可选的编码格式
	CodecType codec.Type
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

// Server RPC服务器
type Server struct{}

// NewServer Server构造函数
func NewServer() *Server {
	return &Server{}
}

// DefaultServer Server实例
var DefaultServer = NewServer()

// Accept 监听服务器请求时接收链接
func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		// 使用多线程链接多个服务
		go server.ServeConn(conn)
	}
}

func Accept(lis net.Listener) { DefaultServer.Accept(lis) }

// ServeConn 服务连接
func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()
	// 使用json反序列化得到option实例
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	// 寻找相应编码格式的解析函数
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	server.serveCodec(f(conn))
}

// 错误发生时响应参数的占位符
var invalidRequest = struct{}{}

// 服务处理
func (server *Server) serveCodec(cc codec.Codec) {
	// 互斥锁
	sending := new(sync.Mutex)
	// 加锁 直到所有请求被处理 等待组
	wg := new(sync.WaitGroup)
	for {
		// 读取请求 一次连接 允许接收多个请求
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				// 不可能恢复 则关闭连接
				break
			}
			req.h.Error = err.Error()
			// 回复请求 逐个发送 使用锁保证
			server.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		// 计数
		wg.Add(1)
		// 处理请求
		go server.handleRequest(cc, req, sending, wg)
	}
	wg.Wait()
	_ = cc.Close()
}

type request struct {
	// 请求头
	h *codec.Header
	// 请求参数和响应值
	argv, replyv reflect.Value
}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	// 不知道argv的类型 暂时假定为string
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
	}
	return req, nil
}

func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	// 加锁
	sending.Lock()
	// 解锁
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	// 需要注册rpc方法到正确的响应中
	// 计数器-1 表示已处理完成一个响应
	defer wg.Done()
	// 只打印header+回复
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq))
	server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}
